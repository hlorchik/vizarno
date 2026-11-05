package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

type fact struct {
	Value       string `json:"value"`
	Source      string `json:"source"`
	Alternative string `json:"alternative,omitempty"`
	OtherSource string `json:"otherSource,omitempty"`
}
type productProfile map[string]fact

func (a app) saveProfile(w http.ResponseWriter, r *http.Request, uid int64) {
	create := r.Method == http.MethodPost
	var id int64
	if !create {
		var ok bool
		id, ok = pathID(w, r)
		if !ok {
			return
		}
	}
	var input struct {
		Title   string
		Profile productProfile
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF || input.Profile == nil {
		problem(w, 400, "Проверьте формат профиля.")
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if create && (input.Title == "" || utf8.RuneCountInString(input.Title) > 150) {
		problem(w, 400, "Введите название до 150 символов.")
		return
	}
	for key, f := range input.Profile {
		if !slices.Contains([]string{"model", "sku", "cpu", "ram", "ssd", "os", "display", "graphics", "weight", "battery", "charging", "ports", "keyboard", "camera", "body", "package"}, key) {
			problem(w, 400, "Неизвестная характеристика.")
			return
		}
		f.Value = strings.TrimSpace(f.Value)
		f.Source = strings.TrimSpace(f.Source)
		f.Alternative = strings.TrimSpace(f.Alternative)
		f.OtherSource = strings.TrimSpace(f.OtherSource)
		if f.Value == "" || f.Source == "" || (f.Alternative == "") != (f.OtherSource == "") || utf8.RuneCountInString(f.Value) > 250 || utf8.RuneCountInString(f.Alternative) > 250 || utf8.RuneCountInString(f.Source) > 500 || utf8.RuneCountInString(f.OtherSource) > 500 {
			problem(w, 400, "Для каждого значения укажите источник. Значение — до 250 символов, источник — до 500.")
			return
		}
		input.Profile[key] = f
	}
	raw, err := json.Marshal(input.Profile)
	if err != nil {
		failed(w, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
		err = a.db.QueryRowContext(r.Context(), "INSERT INTO cards(seller_id,title,profile) VALUES($1,$2,$3) RETURNING id", uid, input.Title, string(raw)).Scan(&id)
	} else {
		err = a.db.QueryRowContext(r.Context(), "UPDATE cards SET profile=$3 WHERE seller_id=$1 AND id=$2 RETURNING id", uid, id, string(raw)).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Карточка не найдена.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	reply(w, status, map[string]any{"id": id, "profile": input.Profile})
}
