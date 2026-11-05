package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const maxGeneratedPages = 8

// Finished images are private seller assets, separate from source versions.
func (a app) renders(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		var exists bool
		if err := a.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM cards WHERE id=$1 AND seller_id=$2)", id, uid).Scan(&exists); err != nil {
			failed(w, err)
			return
		}
		if !exists {
			problem(w, 404, "Карточка не найдена.")
			return
		}
		var raw []byte
		err := a.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('id',g.id,'version',g.version_id,'created',g.created_at,'pages',g.pages,
   'stale',g.profile<>c.profile OR g.version_id<>(SELECT max(v.id) FROM versions v WHERE v.card_id=c.id))
   FROM generated_sets g JOIN cards c ON c.id=g.card_id WHERE c.id=$1 AND c.seller_id=$2 ORDER BY g.id DESC LIMIT 1`, id, uid).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			reply(w, 200, nil)
			return
		}
		if err != nil {
			failed(w, err)
			return
		}
		reply(w, 200, json.RawMessage(raw))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		problem(w, 400, "Не удалось прочитать изображения. Максимум 40 МБ на серию.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	profile := r.FormValue("profile")
	platform := r.FormValue("platform")
	if platform == "" {
		platform = "universal"
	}
	if platform != "universal" && platform != "wildberries" {
		problem(w, 400, "Неизвестное назначение серии.")
		return
	}
	var parsed productProfile
	if err != nil || version < 1 || len(profile) > 65536 || json.Unmarshal([]byte(profile), &parsed) != nil || parsed == nil {
		problem(w, 400, "Проверьте версию и профиль товара.")
		return
	}
	files := r.MultipartForm.File["pages"]
	if len(files) < 1 || len(files) > maxGeneratedPages {
		problem(w, 400, "В серии должно быть от 1 до 8 слайдов.")
		return
	}
	images := make([][]byte, len(files))
	for i, f := range files {
		file, err := f.Open()
		if err != nil {
			problem(w, 400, "Не удалось прочитать PNG.")
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
		file.Close()
		if err != nil || len(data) > 8<<20 {
			problem(w, 400, "Каждый PNG должен занимать не более 8 МБ.")
			return
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != "png" || cfg.Width != 1200 || cfg.Height != 1600 {
			problem(w, 400, "Нужны PNG размером 1200 × 1600.")
			return
		}
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			problem(w, 400, "PNG повреждён.")
			return
		}
		images[i] = data
	}
	var reportJSON []byte
	var currentVersion int64
	var sameProfile bool
	err = a.db.QueryRowContext(r.Context(), `SELECT coalesce(v.id,0),v.report,c.profile=$3::jsonb FROM cards c
 LEFT JOIN LATERAL (SELECT id,report FROM versions WHERE card_id=c.id ORDER BY id DESC LIMIT 1) v ON true
 WHERE c.id=$1 AND c.seller_id=$2`, id, uid, profile).Scan(&currentVersion, &reportJSON, &sameProfile)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	if !sameProfile || currentVersion != version {
		problem(w, 409, "Профиль или версия изменились. Обновите страницу.")
		return
	}
	var audit report
	if err = json.Unmarshal(reportJSON, &audit); err != nil {
		failed(w, err)
		return
	}
	plan := planLayout("", id, version, parsed, audit)
	if len(plan.ReleaseNeeds) > 0 {
		problem(w, 422, strings.Join(plan.ReleaseNeeds, " "))
		return
	}
	if len(images) != len(plan.Steps) {
		problem(w, 422, "Передайте все слайды текущего плана.")
		return
	}
	canonical, _ := json.Marshal(parsed)
	hash := sha256.New()
	fmt.Fprintf(hash, "%x/%d/%d/%d/%s/%x/", methodVersion(), uid, id, version, platform, sha256.Sum256(canonical))
	for _, data := range images {
		part := sha256.Sum256(data)
		hash.Write(part[:])
	}
	digest := hash.Sum(nil)
	checkOnly := strings.HasSuffix(r.URL.Path, "/render-check")
	reviewMode := r.FormValue("review")
	if reviewMode == "" {
		reviewMode = "local"
	}
	if reviewMode != "local" && reviewMode != "ai" {
		problem(w, 400, "Неизвестный режим проверки.")
		return
	}
	var approvedJSON []byte
	var reviewPassed bool
	if checkOnly {
		var review methodReport
		if reviewMode == "ai" {
			var provider aiProvider
			for _, candidate := range providers() {
				if candidate.ID == r.FormValue("provider") && candidate.Enabled {
					provider = candidate
					break
				}
			}
			if provider.ID == "" {
				problem(w, 503, "Для визуальной проверки выберите настроенную нейросеть.")
				return
			}
			review, err = finalMethodReview(r.Context(), &http.Client{Timeout: 34 * time.Second}, provider, os.Getenv(provider.env), audit, parsed, images, platform)
			if err != nil {
				problem(w, 502, err.Error())
				return
			}
		} else {
			review = localReleaseReview(platform, len(images))
		}
		reviewPassed = review.Approved
		approvedJSON, err = json.Marshal(review)
		if err != nil {
			failed(w, err)
			return
		}
		if reviewMode == "ai" {
			reply(w, 200, json.RawMessage(approvedJSON))
			return
		}
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		failed(w, err)
		return
	}
	defer tx.Rollback()
	var matches bool
	err = tx.QueryRowContext(r.Context(), `SELECT profile=$3::jsonb FROM cards WHERE id=$1 AND seller_id=$2 FOR UPDATE`, id, uid, profile).Scan(&matches)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	var latest int64
	if err = tx.QueryRowContext(r.Context(), "SELECT COALESCE(max(id),0) FROM versions WHERE card_id=$1", id).Scan(&latest); err != nil {
		failed(w, err)
		return
	}
	if !matches || latest != version {
		problem(w, 409, "Профиль или исходная обложка изменились. Обновите страницу и создайте слайды заново.")
		return
	}
	if checkOnly {
		if reviewPassed {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO method_approvals(version_id,digest,report) VALUES($1,$2,$3)
 ON CONFLICT(version_id,digest) DO UPDATE SET report=EXCLUDED.report`, version, digest, string(approvedJSON))
		} else {
			_, err = tx.ExecContext(r.Context(), "DELETE FROM method_approvals WHERE version_id=$1 AND digest=$2", version, digest)
		}
		if err != nil {
			failed(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			failed(w, err)
			return
		}
		reply(w, 200, json.RawMessage(approvedJSON))
		return
	}
	err = tx.QueryRowContext(r.Context(), "SELECT report FROM method_approvals WHERE version_id=$1 AND digest=$2", version, digest).Scan(&approvedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 422, "Эти слайды не прошли проверку по методике. Запустите генерацию и проверку.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	var setID int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO generated_sets(card_id,version_id,profile,pages,method_report) VALUES($1,$2,$3,$4,$5) RETURNING id`, id, version, profile, len(images), string(approvedJSON)).Scan(&setID)
	if err != nil {
		failed(w, err)
		return
	}
	for i, data := range images {
		if _, err = tx.ExecContext(r.Context(), "INSERT INTO generated_pages(set_id,page,png) VALUES($1,$2,$3)", setID, i+1, data); err != nil {
			failed(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		failed(w, err)
		return
	}
	reply(w, 201, map[string]any{"id": setID, "version": version, "pages": len(images), "stale": false})
}

func (a app) renderedPage(w http.ResponseWriter, r *http.Request, uid int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	page, err := strconv.Atoi(r.PathValue("page"))
	if err != nil || page < 1 || page > maxGeneratedPages {
		problem(w, 404, "Слайд не найден.")
		return
	}
	var png []byte
	err = a.db.QueryRowContext(r.Context(), `SELECT p.png FROM generated_pages p JOIN generated_sets g ON g.id=p.set_id
 JOIN cards c ON c.id=g.card_id WHERE g.id=$1 AND p.page=$2 AND c.seller_id=$3`, id, page, uid).Scan(&png)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Слайд не найден.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="vizarno-%d-%02d.png"`, id, page))
	}
	w.Write(png)
}
