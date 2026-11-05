package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type seriesSummary struct {
	ID           int64           `json:"id"`
	CardID       int64           `json:"cardId"`
	Title        string          `json:"title"`
	Created      time.Time       `json:"created"`
	Pages        int             `json:"pages"`
	Stale        bool            `json:"stale"`
	MethodReport json.RawMessage `json:"methodReport,omitempty"`
}

const seriesQuery = `SELECT g.id,c.id,c.title,g.created_at,g.pages,
 (g.profile <> c.profile OR g.version_id <> (SELECT max(v.id) FROM versions v WHERE v.card_id=c.id))
 FROM generated_sets g JOIN cards c ON c.id=g.card_id WHERE c.seller_id=$1`

func (s *seriesSummary) scan(row interface{ Scan(...any) error }) error {
	return row.Scan(&s.ID, &s.CardID, &s.Title, &s.Created, &s.Pages, &s.Stale)
}

func (a app) accountSeries(w http.ResponseWriter, r *http.Request, uid int64) {
	if r.PathValue("id") != "" {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var s seriesSummary
		err := s.scan(a.db.QueryRowContext(r.Context(), seriesQuery+" AND g.id=$2", uid, id))
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 404, "Серия не найдена.")
			return
		}
		if err != nil {
			failed(w, err)
			return
		}
		var raw []byte
		err = a.db.QueryRowContext(r.Context(), "SELECT g.method_report FROM generated_sets g JOIN cards c ON c.id=g.card_id WHERE c.seller_id=$1 AND g.id=$2", uid, id).Scan(&raw)
		if err != nil {
			failed(w, err)
			return
		}
		s.MethodReport = raw
		reply(w, 200, s)
		return
	}
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			problem(w, 400, "Неверный номер страницы.")
			return
		}
		before = n
	}
	const pageSize = 12
	rows, err := a.db.QueryContext(r.Context(), seriesQuery+" AND ($2::bigint=0 OR g.id<$2) ORDER BY g.id DESC LIMIT $3", uid, before, pageSize+1)
	if err != nil {
		failed(w, err)
		return
	}
	defer rows.Close()
	list := []seriesSummary{}
	for rows.Next() {
		var s seriesSummary
		if err = s.scan(rows); err != nil {
			failed(w, err)
			return
		}
		list = append(list, s)
	}
	if err = rows.Err(); err != nil {
		failed(w, err)
		return
	}
	var next int64
	if len(list) > pageSize {
		list = list[:pageSize]
		next = list[pageSize-1].ID
	}
	reply(w, 200, map[string]any{"items": list, "next": next})
}
