package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const openAIImageEndpoint = "https://api.openai.com/v1/images/edits"

type aiImagePage struct {
	Page  int    `json:"page"`
	Image string `json:"image"`
}

func imagePrompt(step layoutStep, page, total int, platforms ...string) string {
	platform := "universal"
	if len(platforms) > 0 {
		platform = platforms[0]
	}
	copyText := strings.Join(step.Copy, "\n")
	platformRule := "Это универсальная рекламная серия: напечатай весь переданный текст."
	if platform == "wildberries" {
		copyText = strings.ReplaceAll(copyText, "\nПокупай!", "")
		platformRule = "Это карточка для Wildberries. Не добавляй призывы купить, заказать или добавить товар в корзину, цены, скидки, рейтинг и контакты."
	}
	return fmt.Sprintf(`Создай готовый вертикальный слайд карточки товара для российского маркетплейса.
%s
Это слайд %d из %d. Исходное изображение — единственный источник внешнего вида товара. Сохрани модель, цвет, пропорции, экран, клавиатуру, логотипы, разъёмы и остальные детали товара. Не заменяй товар и не придумывай комплектацию.

Задача слайда: %s
Композиция: %s
Критерий приёмки: %s

Напечатай на изображении только следующий русский текст, без исправлений, дополнений, цен, скидок, рейтингов и выдуманных обещаний:
---
%s
---

Требования: профессиональный чистый дизайн; ясная иерархия; крупный настоящий товар; высокий контраст; текст не перекрывает изделие; все строки читаются; безопасные поля по краям. Не добавляй мелкий декоративный текст. Если описана ситуация использования, окружение служит только фоном и не меняет товар.`, platformRule, page, total, step.Purpose, step.Composition, step.Acceptance, copyText)
}

type imageAPIError struct {
	status     int
	code       string
	message    string
	retryAfter time.Duration
}

func (e *imageAPIError) Error() string { return e.message }

func retryDelay(h http.Header) time.Duration {
	value := strings.TrimSpace(h.Get("Retry-After"))
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && time.Until(when) > 0 {
		return time.Until(when)
	}
	return 0
}

func requestOpenAIImageOnce(ctx context.Context, client *http.Client, endpoint, key, model, mime string, source []byte, prompt string) ([]byte, error) {
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(source)
	payload := map[string]any{
		"model":         model,
		"images":        []map[string]string{{"image_url": dataURL}},
		"prompt":        prompt,
		"size":          "1024x1536",
		"quality":       "medium",
		"output_format": "png",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("OpenAI не ответил на запрос генерации изображения.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Error struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&detail)
		switch resp.StatusCode {
		case 401, 403:
			return nil, errors.New("Проверьте OPENAI_API_KEY и доступ к модели изображений.")
		case 429, 503:
			message := detail.Error.Message
			if message == "" {
				message = "Временный лимит OpenAI."
			}
			return nil, &imageAPIError{status: resp.StatusCode, code: detail.Error.Code, message: message, retryAfter: retryDelay(resp.Header)}
		}
		if detail.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI Images: %s", detail.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI Images вернул HTTP %d", resp.StatusCode)
	}
	var answer struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 24<<20)).Decode(&answer) != nil || len(answer.Data) == 0 || answer.Data[0].B64 == "" {
		return nil, errors.New("OpenAI не вернул готовое изображение.")
	}
	image, err := base64.StdEncoding.DecodeString(answer.Data[0].B64)
	if err != nil || len(image) == 0 {
		return nil, errors.New("OpenAI вернул повреждённое изображение.")
	}
	return image, nil
}

func requestOpenAIImage(ctx context.Context, client *http.Client, endpoint, key, model, mime string, source []byte, prompt string) ([]byte, error) {
	for attempt := 0; attempt < 4; attempt++ {
		image, err := requestOpenAIImageOnce(ctx, client, endpoint, key, model, mime, source, prompt)
		if err == nil {
			return image, nil
		}
		var apiErr *imageAPIError
		if !errors.As(err, &apiErr) {
			return nil, err
		}
		switch apiErr.code {
		case "credit_balance_exhausted":
			return nil, errors.New("На балансе OpenAI API закончились средства. Пополните API Credits в Platform → Billing.")
		case "organization_spend_limit_exceeded", "project_spend_limit_exceeded":
			return nil, errors.New("Достигнут лимит расходов OpenAI API. Увеличьте лимит проекта в Platform → Billing.")
		case "organization_usage_limit_exceeded":
			return nil, errors.New("Достигнут лимит использования OpenAI API для организации.")
		}
		if attempt == 3 {
			return nil, errors.New("Временный лимит OpenAI не снялся после повторов. Подождите несколько минут и запустите снова.")
		}
		delay := time.Duration(8*(1<<attempt)) * time.Second
		if apiErr.retryAfter > delay {
			delay = apiErr.retryAfter
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, errors.New("Не удалось создать изображение.")
}

func (a app) generateAIImages(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Version  int64  `json:"version"`
		Start    int    `json:"start"`
		Platform string `json:"platform"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.Version < 1 {
		problem(w, 400, "Проверьте версию исходной карточки.")
		return
	}
	if input.Platform == "" {
		input.Platform = "universal"
	}
	if input.Platform != "universal" && input.Platform != "wildberries" {
		problem(w, 400, "Неизвестное назначение серии.")
		return
	}
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		problem(w, 503, "Добавьте OPENAI_API_KEY в .env сервера.")
		return
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_IMAGE_MODEL"))
	if model == "" {
		model = "gpt-image-2.5-sunburst"
	}

	var latest, version int64
	var title, mime string
	var profileRaw, reportRaw, cover []byte
	err := a.db.QueryRowContext(r.Context(), `SELECT c.title,c.profile,v.id,v.report,v.mime,v.cover,
	 (SELECT max(v2.id) FROM versions v2 WHERE v2.card_id=c.id)
	 FROM cards c JOIN versions v ON v.card_id=c.id AND v.id=$3
	 WHERE c.id=$1 AND c.seller_id=$2`, id, uid, input.Version).Scan(&title, &profileRaw, &version, &reportRaw, &mime, &cover, &latest)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Карточка или версия не найдена.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	if latest != version {
		problem(w, 409, "Исходная карточка изменилась. Обновите страницу.")
		return
	}
	var profile productProfile
	var audit report
	if json.Unmarshal(profileRaw, &profile) != nil || json.Unmarshal(reportRaw, &audit) != nil {
		failed(w, errors.New("invalid stored generation data"))
		return
	}
	plan := planLayout(title, id, version, profile, audit)
	if len(plan.ReleaseNeeds) > 0 {
		problem(w, 422, strings.Join(plan.ReleaseNeeds, " "))
		return
	}
	if len(plan.Steps) < 1 || len(plan.Steps) > 8 {
		problem(w, 422, "План должен содержать от 1 до 8 слайдов.")
		return
	}
	if input.Start == 0 {
		input.Start = 1
	}
	if input.Start < 1 || input.Start > len(plan.Steps) {
		problem(w, 400, "Неверный номер продолжения серии.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 14*time.Minute)
	defer cancel()
	pages := make([]aiImagePage, 0, len(plan.Steps)-input.Start+1)
	client := &http.Client{Timeout: 4 * time.Minute}
	for i := input.Start - 1; i < len(plan.Steps); i++ {
		step := plan.Steps[i]
		image, err := requestOpenAIImage(ctx, client, openAIImageEndpoint, key, model, mime, cover, imagePrompt(step, i+1, len(plan.Steps), input.Platform))
		if err != nil {
			reply(w, 200, map[string]any{"model": model, "pages": pages, "complete": false, "error": fmt.Sprintf("Слайд %d: %v", i+1, err)})
			return
		}
		pages = append(pages, aiImagePage{Page: i + 1, Image: base64.StdEncoding.EncodeToString(image)})
	}
	reply(w, 200, map[string]any{"model": model, "pages": pages, "complete": true})
}
