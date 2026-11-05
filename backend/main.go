package main

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/lib/pq"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

type app struct{ db *sql.DB }

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, s int, m string) { reply(w, s, map[string]string{"error": m}) }
func failed(w http.ResponseWriter, err error) {
	log.Print(err)
	problem(w, 500, "Не удалось выполнить операцию. Попробуйте ещё раз.")
}
func (a app) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/questions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(questionsJSON)
	})
	m.HandleFunc("POST /api/register", a.auth)
	m.HandleFunc("POST /api/login", a.auth)
	m.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, e := r.Cookie("session"); e == nil {
			d := sha256.Sum256([]byte(c.Value))
			if _, e = a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE digest=$1", d[:]); e != nil {
				failed(w, e)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1, HttpOnly: true})
		w.WriteHeader(204)
	})
	private := func(pattern string, handler func(http.ResponseWriter, *http.Request, int64)) {
		m.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "private, no-store")
			c, e := r.Cookie("session")
			if e != nil {
				problem(w, 401, "Войдите в кабинет.")
				return
			}
			d := sha256.Sum256([]byte(c.Value))
			var uid int64
			e = a.db.QueryRowContext(r.Context(), "SELECT seller_id FROM sessions WHERE digest=$1 AND expires_at>now()", d[:]).Scan(&uid)
			if errors.Is(e, sql.ErrNoRows) {
				problem(w, 401, "Войдите в кабинет.")
				return
			}
			if e != nil {
				failed(w, e)
				return
			}
			handler(w, r, uid)
		})
	}
	private("GET /api/account", func(w http.ResponseWriter, r *http.Request, id int64) {
		var email string
		if err := a.db.QueryRowContext(r.Context(), "SELECT email FROM sellers WHERE id=$1", id).Scan(&email); err != nil {
			failed(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]string{"email": email})
	})
	private("DELETE /api/account", a.deleteAccount)
	private("POST /api/account/password", a.changePassword)
	private("GET /api/ai/providers", providerOptions)
	private("POST /api/cards/{id}/ai-brief", a.designerSpec)
	private("GET /api/cards/{id}/designer-brief", a.designerSpec)
	private("GET /api/cards", a.cards)
	private("POST /api/products", a.saveProfile)
	private("PUT /api/cards/{id}/profile", a.saveProfile)
	private("GET /api/cards/{id}/renders", a.renders)
	private("GET /api/account/series", a.accountSeries)
	private("GET /api/account/series/{id}", a.accountSeries)
	private("POST /api/cards/{id}/renders", a.renders)
	private("POST /api/cards/{id}/render-check", a.renders)
	private("POST /api/cards/{id}/ai-images", a.generateAIImages)
	private("GET /api/renders/{id}/pages/{page}", a.renderedPage)
	private("POST /api/cards", a.upload)
	private("GET /api/cards/{id}/versions", a.history)
	private("POST /api/cards/{id}/versions", a.upload)
	private("GET /api/covers/{id}", a.cover)
	private("GET /api/covers/{id}/slides/{slide}", a.cover)
	m.Handle("/", http.FileServer(http.Dir("public")))
	return http.NewCrossOriginProtection().Handler(m)
}
func (a app) auth(w http.ResponseWriter, r *http.Request) {
	var v struct{ Email, Password string }
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		problem(w, 400, "Проверьте данные формы.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(v.Email))
	addr, e := mail.ParseAddress(email)
	if e != nil || addr.Address != email || len(email) > 254 || len(v.Password) < 8 || len(v.Password) > 256 {
		problem(w, 400, "Введите email и пароль длиной от 8 до 256 байт.")
		return
	}
	ctx := r.Context()
	var id int64
	var salt, hash []byte
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		failed(w, e)
		return
	}
	defer tx.Rollback()
	if r.URL.Path == "/api/register" {
		salt = make([]byte, 16)
		rand.Read(salt)
		e = tx.QueryRowContext(ctx, "INSERT INTO sellers(email,salt,password_hash) VALUES($1,$2,$3) RETURNING id", email, salt, passwordHash(v.Password, salt)).Scan(&id)
		var pe *pq.Error
		if errors.As(e, &pe) && pe.Code == "23505" {
			problem(w, 409, "Этот email уже зарегистрирован.")
			return
		}
	} else {
		e = tx.QueryRowContext(ctx, "SELECT id,salt,password_hash FROM sellers WHERE email=$1 FOR SHARE", email).Scan(&id, &salt, &hash)
		if errors.Is(e, sql.ErrNoRows) || (e == nil && subtle.ConstantTimeCompare(passwordHash(v.Password, salt), hash) != 1) {
			problem(w, 401, "Проверьте email и пароль.")
			return
		}
	}
	if e != nil {
		failed(w, e)
		return
	}
	token := make([]byte, 32)
	rand.Read(token)
	raw := hex.EncodeToString(token)
	d := sha256.Sum256([]byte(raw))
	_, e = tx.ExecContext(ctx, "INSERT INTO sessions(digest,seller_id,expires_at) VALUES($1,$2,now()+interval '7 days')", d[:], id)
	if e != nil {
		failed(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		failed(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: raw, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: os.Getenv("COOKIE_SECURE") == "true", MaxAge: 604800})
	reply(w, 200, map[string]string{"email": email})
}
func passwordHash(password string, salt []byte) []byte {
	hash, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil {
		panic(err)
	}
	return hash
}

func (a app) changePassword(w http.ResponseWriter, r *http.Request, id int64) {
	var input struct{ Current, New string }
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.New) < 8 || len(input.New) > 256 || input.Current == input.New {
		problem(w, 400, "Введите другой пароль длиной от 8 до 256 байт.")
		return
	}
	ctx := r.Context()
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		failed(w, e)
		return
	}
	defer tx.Rollback()
	var salt, hash []byte
	e = tx.QueryRowContext(ctx, "SELECT salt,password_hash FROM sellers WHERE id=$1 FOR UPDATE", id).Scan(&salt, &hash)
	if e != nil {
		failed(w, e)
		return
	}
	if subtle.ConstantTimeCompare(passwordHash(input.Current, salt), hash) != 1 {
		problem(w, 403, "Текущий пароль неверен.")
		return
	}
	salt = make([]byte, 16)
	rand.Read(salt)
	if _, e = tx.ExecContext(ctx, "UPDATE sellers SET salt=$2,password_hash=$3 WHERE id=$1", id, salt, passwordHash(input.New, salt)); e != nil {
		failed(w, e)
		return
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE seller_id=$1", id); e != nil {
		failed(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		failed(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", HttpOnly: true, MaxAge: -1})
	w.WriteHeader(204)
}

func (a app) deleteAccount(w http.ResponseWriter, r *http.Request, id int64) {
	var input struct{ Password string }
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.Password == "" {
		problem(w, 400, "Введите текущий пароль.")
		return
	}
	ctx := r.Context()
	var salt, hash []byte
	err := a.db.QueryRowContext(ctx, "SELECT salt,password_hash FROM sellers WHERE id=$1", id).Scan(&salt, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 401, "Войдите в кабинет.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	if subtle.ConstantTimeCompare(passwordHash(input.Password, salt), hash) != 1 {
		problem(w, 403, "Текущий пароль неверен.")
		return
	}
	// Compare again in the DELETE to cover a concurrent password change.
	err = a.db.QueryRowContext(ctx, "DELETE FROM sellers WHERE id=$1 AND password_hash=$2 RETURNING id", id, hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 409, "Данные аккаунта изменились. Войдите снова.")
		return
	}
	if err != nil {
		failed(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", HttpOnly: true, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	db, e := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, e = db.ExecContext(ctx, schema); e != nil {
		log.Fatal(e)
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		addr = ":" + port
	}
	log.Print("Визарно ", addr)
	s := http.Server{Addr: addr, Handler: app{db}.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 15 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Fatal(s.ListenAndServe())
}
