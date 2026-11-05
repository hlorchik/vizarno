package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type savedAnalysis struct {
	Fingerprint string       `json:"fingerprint"`
	Tasks       []string     `json:"tasks"`
	Plan        creativePlan `json:"plan"`
	Text        string       `json:"text"`
	Provider    string       `json:"provider"`
	Model       string       `json:"model"`
	Review      visualReview `json:"review"`
}

func (a app) designerSpec(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var title string
	var profileJSON, reportJSON, savedJSON []byte
	var version sql.NullInt64
	err := a.db.QueryRowContext(r.Context(), `SELECT c.title,c.profile,v.id,v.report,v.ai_result
 FROM cards c LEFT JOIN LATERAL (SELECT id,report,ai_result FROM versions WHERE card_id=c.id ORDER BY id DESC LIMIT 1) v ON true
 WHERE c.id=$1 AND c.seller_id=$2`, id, uid).Scan(&title, &profileJSON, &version, &reportJSON, &savedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	var profile productProfile
	if err = json.Unmarshal(profileJSON, &profile); err != nil {
		failed(w, err)
		return
	}
	var audit report
	if version.Valid {
		if err = json.Unmarshal(reportJSON, &audit); err != nil {
			failed(w, err)
			return
		}
	}
	plan := planLayout(title, id, version.Int64, profile, audit)
	if wanted := r.URL.Query().Get("version"); wanted != "" && wanted != strconv.FormatInt(version.Int64, 10) {
		problem(w, 409, "Появилась другая версия карточки. Обновите страницу.")
		return
	}
	var findings []finding
	var saved *savedAnalysis
	if len(savedJSON) > 0 {
		if err = json.Unmarshal(savedJSON, &saved); err != nil {
			failed(w, err)
			return
		}
	}
	fingerprint := fmt.Sprintf("brief-v4:%x:%x", methodVersion(), sha256.Sum256(profileJSON))
	if saved != nil && saved.Fingerprint != fingerprint {
		saved = nil
	}
	var taskIDs []string
	if version.Valid {
		plan.Materials = &audit.Materials
		findings = analyze(audit, profile)
		selected := strings.Split(r.URL.Query().Get("tasks"), ",")
		explicit := r.URL.Query().Has("tasks")
		if !explicit && saved != nil && r.URL.Query().Get("format") == "preview" {
			selected = saved.Tasks
			explicit = true
		}
		for _, f := range findings {
			if (!explicit && f.Status == "fix") || (f.Status != "ok" && slices.Contains(selected, f.ID)) {
				plan.Tasks = append(plan.Tasks, f)
				taskIDs = append(taskIDs, f.ID)
			}
		}
	}
	if r.Method == http.MethodPost {
		if !version.Valid {
			problem(w, 400, "Сначала загрузите обложку.")
			return
		}
		idProvider := r.URL.Query().Get("provider")
		if idProvider == "" {
			idProvider = "openai"
		}
		var provider aiProvider
		for _, candidate := range providers() {
			if candidate.ID == idProvider {
				provider = candidate
				break
			}
		}
		if provider.ID == "" {
			problem(w, 400, "Неизвестный провайдер.")
			return
		}
		key := os.Getenv(provider.env)
		if key == "" {
			problem(w, 503, "Добавьте "+provider.env+" в .env сервера.")
			return
		}
		var mime string
		var cover []byte
		err = a.db.QueryRowContext(r.Context(), "SELECT v.mime,v.cover FROM versions v JOIN cards c ON c.id=v.card_id WHERE v.id=$1 AND c.seller_id=$2", version.Int64, uid).Scan(&mime, &cover)
		if err != nil {
			failed(w, err)
			return
		}
		rows, err := a.db.QueryContext(r.Context(), "SELECT mime,image FROM gallery WHERE version_id=$1 ORDER BY slide_no", version.Int64)
		if err != nil {
			failed(w, err)
			return
		}
		var slides []visualImage
		defer rows.Close()
		for rows.Next() {
			var s visualImage
			if err = rows.Scan(&s.Mime, &s.Data); err != nil {
				failed(w, err)
				return
			}
			slides = append(slides, s)
		}
		if err = rows.Err(); err != nil {
			failed(w, err)
			return
		}
		review, err := reviewCover(r.Context(), &http.Client{Timeout: 35 * time.Second}, key, provider, plan, mime, cover, slides...)
		if err != nil {
			problem(w, 502, err.Error())
			return
		}
		plan.Source += "\nНаблюдения ИИ: " + review.Observation
		if review.OmittedInsights > 0 {
			plan.Source += fmt.Sprintf("\nЗамечаний ИИ без подтверждённого источника исключено: %d.", review.OmittedInsights)
		}
		for i, note := range review.Gallery {
			plan.Source += fmt.Sprintf("\nСлайд %d · наблюдение ИИ: %s", i+2, note)
		}
		for i, design := range review.Designs {
			plan.Steps[i].Composition = design.composition(plan.Steps[i].Copy)
		}
		result := savedAnalysis{fingerprint, taskIDs, plan, plan.text(), provider.Label, provider.Model, review}
		raw, err := json.Marshal(result)
		if err != nil {
			failed(w, err)
			return
		}
		if _, err = a.db.ExecContext(r.Context(), "UPDATE versions SET ai_result=$2 WHERE id=$1", version.Int64, string(raw)); err != nil {
			failed(w, err)
			return
		}
		reply(w, 200, result)
		return
	}
	if r.URL.Query().Get("format") == "preview" {
		if saved != nil && !slices.Equal(saved.Tasks, taskIDs) {
			saved = nil
		}
		reply(w, 200, map[string]any{"plan": plan, "text": plan.text(), "findings": findings, "saved": saved})
		return
	}
	if r.URL.Query().Get("format") == "json" {
		reply(w, http.StatusOK, plan)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="vizarno-brief-%d.txt"`, id))
	fmt.Fprint(w, plan.text())
}
