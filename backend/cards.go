package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type check struct {
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	Note  string `json:"note"`
}
type report struct {
	Materials materials `json:"materials"`
	Findings  []finding `json:"findings,omitempty"`
	Brief     *brief    `json:"brief,omitempty"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Bytes     int       `json:"bytes"`
	Checks    []check   `json:"checks"`
}

// Limits are internal editorial targets; no marketplace compliance or conversion claim.
func inspect(data []byte) (report, string, error) {
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "jpeg" && format != "png") {
		return report{}, "", errors.New("Выберите изображение JPEG или PNG.")
	}
	if int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return report{}, "", errors.New("Максимум 16 мегапикселей.")
	}
	if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
		return report{}, "", errors.New("Файл повреждён. Экспортируйте обложку заново.")
	}
	large := min(cfg.Width, cfg.Height) >= 800
	light := len(data) <= 2*1024*1024
	detail := fmt.Sprintf("%d × %d пикселей. Ориентир мастерской — от 800 по короткой стороне.", cfg.Width, cfg.Height)
	if !large {
		detail += " Экспортируйте из исходника большего размера."
	}
	weight := fmt.Sprintf("%.2f МБ. Рабочий ориентир — до 2 МБ.", float64(len(data))/(1024*1024))
	if !light {
		weight += " Попробуйте уменьшить вес при экспорте."
	}
	return report{Width: cfg.Width, Height: cfg.Height, Bytes: len(data), Checks: []check{{"Разрешение", large, detail}, {"Вес файла", light, weight}}}, "image/" + format, nil
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		problem(w, 404, "Запись не найдена.")
		return 0, false
	}
	return id, true
}
func (a app) cards(w http.ResponseWriter, r *http.Request, uid int64) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,title,profile FROM cards WHERE seller_id=$1 ORDER BY id DESC", uid)
	if e != nil {
		failed(w, e)
		return
	}
	defer rows.Close()
	type card struct {
		ID      int64           `json:"id"`
		Title   string          `json:"title"`
		Profile json.RawMessage `json:"profile"`
	}
	list := []card{}
	for rows.Next() {
		var c card
		if e = rows.Scan(&c.ID, &c.Title, &c.Profile); e != nil {
			failed(w, e)
			return
		}
		list = append(list, c)
	}
	if e = rows.Err(); e != nil {
		failed(w, e)
		return
	}
	reply(w, 200, list)
}
func (a app) upload(w http.ResponseWriter, r *http.Request, uid int64) {
	const limit = 8 << 20
	r.Body = http.MaxBytesReader(w, r.Body, 5*limit+(128<<10))
	if e := r.ParseMultipartForm(limit); e != nil {
		problem(w, 400, "Проверьте форму. Максимальный размер файла — 8 МБ.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	newCard := r.PathValue("id") == ""
	title := strings.TrimSpace(r.FormValue("title"))
	var id int64
	if newCard {
		if title == "" || utf8.RuneCountInString(title) > 150 {
			problem(w, 400, "Введите название до 150 символов.")
			return
		}
	} else {
		var ok bool
		if id, ok = pathID(w, r); !ok {
			return
		}
	}
	var data []byte
	var mime string
	var result report
	var previousVersion int64
	var previousReport report
	f, _, e := r.FormFile("cover")
	if e == nil {
		defer f.Close()
		data, e = io.ReadAll(io.LimitReader(f, limit+1))
		if e != nil || len(data) > limit {
			problem(w, 400, "Не удалось прочитать файл до 8 МБ.")
			return
		}
		result, mime, e = inspect(data)
	} else if !newCard && errors.Is(e, http.ErrMissingFile) {
		var raw []byte
		e = a.db.QueryRowContext(r.Context(), `SELECT v.id,v.mime,v.cover,v.report
			FROM versions v JOIN cards c ON c.id=v.card_id
			WHERE c.id=$1 AND c.seller_id=$2 ORDER BY v.id DESC LIMIT 1`, id, uid).Scan(&previousVersion, &mime, &data, &raw)
		if errors.Is(e, sql.ErrNoRows) {
			problem(w, 400, "Добавьте первую обложку товара.")
			return
		}
		if e == nil {
			e = json.Unmarshal(raw, &previousReport)
		}
		if e == nil {
			result, _, e = inspect(data)
		}
	} else {
		problem(w, 400, "Добавьте обложку.")
		return
	}
	if e != nil {
		problem(w, 400, e.Error())
		return
	}
	result.Brief, e = makeBrief(r)
	if e != nil {
		problem(w, 400, e.Error())
		return
	}
	result.Materials.Headline = strings.TrimSpace(r.FormValue("headline"))
	result.Materials.Description = strings.TrimSpace(r.FormValue("description"))
	if utf8.RuneCountInString(result.Materials.Headline) > 300 || utf8.RuneCountInString(result.Materials.Description) > 6000 {
		problem(w, 400, "Заголовок — до 300 символов, описание — до 6000.")
		return
	}
	files := r.MultipartForm.File["gallery"]
	captions := r.MultipartForm.Value["caption"]
	if len(files) > 4 || len(captions) > len(files) {
		problem(w, 400, "Добавьте до четырёх слайдов, по одной подписи на файл.")
		return
	}
	if len(files) == 0 && !newCard && previousVersion == 0 {
		var raw []byte
		e = a.db.QueryRowContext(r.Context(), `SELECT v.id,v.report FROM versions v JOIN cards c ON c.id=v.card_id
			WHERE c.id=$1 AND c.seller_id=$2 ORDER BY v.id DESC LIMIT 1`, id, uid).Scan(&previousVersion, &raw)
		if e == nil {
			e = json.Unmarshal(raw, &previousReport)
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			failed(w, e)
			return
		}
	}
	var slides []visualImage
	if len(files) == 0 && previousVersion > 0 {
		result.Materials.Slides = previousReport.Materials.Slides
	}
	for i, file := range files {
		if file.Size > limit {
			problem(w, 400, "Каждый слайд должен быть до 8 МБ.")
			return
		}
		stream, err := file.Open()
		if err != nil {
			failed(w, err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(stream, limit+1))
		stream.Close()
		if err != nil || len(data) > limit {
			problem(w, 400, "Не удалось прочитать слайд.")
			return
		}
		check, mime, err := inspect(data)
		if err != nil {
			problem(w, 400, fmt.Sprintf("Слайд %d: %s", i+2, err))
			return
		}
		caption := ""
		if i < len(captions) {
			caption = strings.TrimSpace(captions[i])
		}
		if utf8.RuneCountInString(caption) > 600 {
			problem(w, 400, "Подпись слайда — до 600 символов.")
			return
		}
		result.Materials.Slides = append(result.Materials.Slides, slideInfo{check.Width, check.Height, caption})
		slides = append(slides, visualImage{mime, data})
	}
	ctx := r.Context()
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		failed(w, e)
		return
	}
	defer tx.Rollback()
	if newCard {
		e = tx.QueryRowContext(ctx, "INSERT INTO cards(seller_id,title) VALUES($1,$2) RETURNING id", uid, title).Scan(&id)
		if e != nil {
			failed(w, e)
			return
		}
	}
	var version int64
	var profileJSON []byte
	e = tx.QueryRowContext(ctx, "SELECT profile FROM cards WHERE id=$1 AND seller_id=$2 FOR UPDATE", id, uid).Scan(&profileJSON)
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if e != nil {
		failed(w, e)
		return
	}
	var profile productProfile
	if e = json.Unmarshal(profileJSON, &profile); e != nil {
		failed(w, e)
		return
	}
	result.Findings = analyze(result, profile)
	raw, e := json.Marshal(result)
	if e != nil {
		failed(w, e)
		return
	}
	e = tx.QueryRowContext(ctx, "INSERT INTO versions(card_id,mime,cover,report) SELECT id,$3,$4,$5 FROM cards WHERE id=$1 AND seller_id=$2 RETURNING id", id, uid, mime, data, string(raw)).Scan(&version)
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if e != nil {
		failed(w, e)
		return
	}
	for i, s := range slides {
		if _, e = tx.ExecContext(ctx, "INSERT INTO gallery(version_id,slide_no,mime,image) VALUES($1,$2,$3,$4)", version, i+2, s.Mime, s.Data); e != nil {
			failed(w, e)
			return
		}
	}
	if len(slides) == 0 && previousVersion > 0 {
		if _, e = tx.ExecContext(ctx, `INSERT INTO gallery(version_id,slide_no,mime,image)
			SELECT $1,slide_no,mime,image FROM gallery WHERE version_id=$2`, version, previousVersion); e != nil {
			failed(w, e)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		failed(w, e)
		return
	}
	reply(w, 201, map[string]int64{"id": id, "version": version})
}
func (a app) history(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT v.id,v.created_at,v.report FROM versions v JOIN cards c ON c.id=v.card_id WHERE c.id=$1 AND c.seller_id=$2 ORDER BY v.id DESC", id, uid)
	if e != nil {
		failed(w, e)
		return
	}
	defer rows.Close()
	type version struct {
		ID      int64           `json:"id"`
		Created time.Time       `json:"created"`
		Report  json.RawMessage `json:"report"`
	}
	list := []version{}
	for rows.Next() {
		var v version
		if e = rows.Scan(&v.ID, &v.Created, &v.Report); e != nil {
			failed(w, e)
			return
		}
		list = append(list, v)
	}
	if e = rows.Err(); e != nil {
		failed(w, e)
		return
	}
	if len(list) == 0 {
		var exists bool
		if e = a.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT FROM cards WHERE id=$1 AND seller_id=$2)", id, uid).Scan(&exists); e != nil {
			failed(w, e)
			return
		}
		if !exists {
			problem(w, 404, "Карточка не найдена.")
			return
		}
	}
	reply(w, 200, list)
}
func (a app) cover(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var mime string
	var data []byte
	query := "SELECT v.mime,v.cover FROM versions v JOIN cards c ON c.id=v.card_id WHERE v.id=$1 AND c.seller_id=$2"
	args := []any{id, uid}
	if slide := r.PathValue("slide"); slide != "" {
		n, err := strconv.Atoi(slide)
		if err != nil || n < 2 || n > 5 {
			problem(w, 404, "Слайд не найден.")
			return
		}
		query = "SELECT g.mime,g.image FROM gallery g JOIN versions v ON v.id=g.version_id JOIN cards c ON c.id=v.card_id WHERE v.id=$1 AND c.seller_id=$2 AND g.slide_no=$3"
		args = append(args, n)
	}
	e := a.db.QueryRowContext(r.Context(), query, args...).Scan(&mime, &data)
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 404, "Обложка не найдена.")
		return
	}
	if e != nil {
		failed(w, e)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
