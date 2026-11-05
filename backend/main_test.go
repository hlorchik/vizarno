package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func sample(t *testing.T, n int) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewGray(image.Rect(0, 0, n, n))); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestInspect(t *testing.T) {
	for _, n := range []int{320, 800} {
		r, _, e := inspect(sample(t, n))
		if e != nil || r.Checks[0].OK != (n >= 800) {
			t.Fatalf("width %d: %+v %v", n, r, e)
		}
	}
	b := sample(t, 80)
	if _, _, e := inspect(b[:len(b)/2]); e == nil {
		t.Fatal("accepted truncated image")
	}
}
func TestSellerWorkflow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(schema); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(app{db}.routes())
	defer server.Close()
	client := func() *http.Client { jar, _ := cookiejar.New(nil); return &http.Client{Jar: jar} }
	first, second := client(), client()
	call := func(c *http.Client, method, path, content string, body io.Reader, status int) []byte {
		t.Helper()
		req, e := http.NewRequest(method, server.URL+"/api/"+path, body)
		if e != nil {
			t.Fatal(e)
		}
		if content != "" {
			req.Header.Set("Content-Type", content)
		}
		res, e := c.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, e := io.ReadAll(res.Body)
		if e != nil || res.StatusCode != status {
			t.Fatalf("%s %s: %d %s %v", method, path, res.StatusCode, data, e)
		}
		return data
	}
	account := func(suffix string) string {
		return fmt.Sprintf(`{"Email":"qa-%d-%s@example.test","Password":"test-example-123"}`, time.Now().UnixNano(), suffix)
	}
	a, b := account("a"), account("b")
	call(first, "GET", "account", "", nil, 401)
	call(first, "GET", "cards", "", nil, 401)
	call(first, "POST", "register", "application/json", strings.NewReader(a), 200)
	call(second, "POST", "register", "application/json", strings.NewReader(b), 200)
	for session, credentials := range map[*http.Client]string{first: a, second: b} {
		var expected, profile struct{ Email string }
		if err := json.Unmarshal([]byte(credentials), &expected); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(call(session, "GET", "account", "", nil, 200), &profile); err != nil || profile.Email != expected.Email {
			t.Fatalf("wrong account: %q; expected %q", profile.Email, expected.Email)
		}
	}
	upload := func(c *http.Client, path string, n, status int) map[string]int64 {
		t.Helper()
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		w.WriteField("title", "Чашка")
		f, _ := w.CreateFormFile("cover", "cover.png")
		f.Write(sample(t, n))
		w.Close()
		data := call(c, "POST", path, w.FormDataContentType(), &buf, status)
		var ids map[string]int64
		if status == 201 {
			if e := json.Unmarshal(data, &ids); e != nil {
				t.Fatal(e)
			}
		}
		return ids
	}
	draftPayload := `{"title":"Huawei","profile":{"os":{"value":"без ОС","source":"заголовок"}}}`
	var draft struct{ ID int64 }
	if err := json.Unmarshal(call(first, "POST", "products", "application/json", strings.NewReader(draftPayload), 201), &draft); err != nil {
		t.Fatal(err)
	}
	profilePath := fmt.Sprintf("cards/%d/profile", draft.ID)
	call(second, "PUT", profilePath, "application/json", strings.NewReader(draftPayload), 404)
	call(first, "PUT", profilePath, "application/json", strings.NewReader(`{"profile":{"os":{"value":"Linux"}}}`), 400)
	call(first, "PUT", profilePath, "application/json", strings.NewReader(`{"profile":{"os":{"value":"без ОС","source":"заголовок","alternative":"Linux","otherSource":"описание"}}}`), 200)
	var drafts []struct {
		ID      int64
		Profile productProfile
	}
	if err := json.Unmarshal(call(first, "GET", "cards", "", nil, 200), &drafts); err != nil || len(drafts) != 1 || drafts[0].Profile["os"].OtherSource != "описание" {
		t.Fatalf("draft: %+v %v", drafts, err)
	}
	if data := call(first, "GET", fmt.Sprintf("cards/%d/versions", draft.ID), "", nil, 200); strings.TrimSpace(string(data)) != "[]" {
		t.Fatal("expected empty versions")
	}
	ids := upload(first, "cards", 360, 201)
	briefPath := fmt.Sprintf("cards/%d/designer-brief", draft.ID)
	call(second, "GET", briefPath, "", nil, 404)
	call(client(), "GET", briefPath, "", nil, 401)
	draftText := string(call(first, "GET", briefPath, "", nil, 200))
	if !strings.Contains(draftText, "согласовать до публикации") || !strings.Contains(draftText, "Обложка ещё не загружена") {
		t.Fatal("incomplete draft brief", draftText)
	}
	var plan creativePlan
	if err := json.Unmarshal(call(first, "GET", briefPath+"?format=json", "", nil, 200), &plan); err != nil || plan.text() != draftText || len(plan.Steps) != 1 {
		t.Fatalf("designer and generator plans differ: %v", err)
	}
	exportPath := fmt.Sprintf("cards/%d/designer-brief", ids["id"])
	if !strings.Contains(string(call(first, "GET", exportPath, "", nil, 200)), "360 × 360") {
		t.Fatal("missing audit")
	}
	path := fmt.Sprintf("cards/%d/versions", ids["id"])
	cover := fmt.Sprintf("covers/%d", ids["version"])
	if data := call(first, "GET", cover, "", nil, 200); !bytes.Equal(data, sample(t, 360)) {
		t.Fatal("image changed")
	}
	call(second, "GET", cover, "", nil, 404)
	call(second, "GET", path, "", nil, 404)
	upload(second, path, 900, 404)
	if data := call(second, "GET", "cards", "", nil, 200); strings.TrimSpace(string(data)) != "[]" {
		t.Fatal("leaked card")
	}
	upload(first, path, 900, 201)
	if !strings.Contains(string(call(first, "GET", exportPath, "", nil, 200)), "900 × 900") {
		t.Fatal("stale audit")
	}
	var inherited bytes.Buffer
	inheritedForm := multipart.NewWriter(&inherited)
	inheritedForm.WriteField("headline", "Обновлённый текст без нового изображения")
	inheritedForm.WriteField("questionnaire", "yes")
	inheritedForm.Close()
	var inheritedIDs map[string]int64
	if err := json.Unmarshal(call(first, "POST", path, inheritedForm.FormDataContentType(), &inherited, 201), &inheritedIDs); err != nil {
		t.Fatal(err)
	}
	if data := call(first, "GET", fmt.Sprintf("covers/%d", inheritedIDs["version"]), "", nil, 200); !bytes.Equal(data, sample(t, 900)) {
		t.Fatal("previous cover was not inherited")
	}
	call(first, "POST", "logout", "", nil, 204)
	call(first, "GET", path, "", nil, 401)
	call(first, "POST", "login", "application/json", strings.NewReader(a), 200)
	var versions []struct{ Report report }
	if e = json.Unmarshal(call(first, "GET", path, "", nil, 200), &versions); e != nil || len(versions) != 3 || versions[0].Report.Width != 900 || versions[1].Report.Width != 900 || versions[2].Report.Width != 360 {
		t.Fatalf("history %+v, %v", versions, e)
	}

	other := client()
	call(other, "POST", "login", "application/json", strings.NewReader(a), 200)
	call(first, "POST", "account/password", "application/json", strings.NewReader(`{"Current":"wrong","New":"new-example-123"}`), 403)
	call(first, "POST", "account/password", "application/json", strings.NewReader(`{"Current":"test-example-123","New":"new-example-123"}`), 204)
	call(other, "GET", "cards", "", nil, 401)
	call(first, "GET", "cards", "", nil, 401)
	call(first, "POST", "login", "application/json", strings.NewReader(a), 401)
	call(first, "POST", "login", "application/json", strings.NewReader(strings.Replace(a, "test-example-123", "new-example-123", 1)), 200)
	call(second, "GET", "cards", "", nil, 200)

	credentials := strings.Replace(a, "test-example-123", "new-example-123", 1)
	call(other, "POST", "login", "application/json", strings.NewReader(credentials), 200)
	call(first, "DELETE", "account", "application/json", strings.NewReader(`{"password":"wrong"}`), 403)
	call(first, "GET", path, "", nil, 200)
	call(first, "DELETE", "account", "application/json", strings.NewReader(`{"password":"new-example-123"}`), 204)
	call(other, "GET", "cards", "", nil, 401)
	call(first, "POST", "login", "application/json", strings.NewReader(credentials), 401)
	call(second, "GET", "cards", "", nil, 200)
	var gone bool
	if err := db.QueryRow("SELECT NOT EXISTS(SELECT FROM cards WHERE id=$1) AND NOT EXISTS(SELECT FROM versions WHERE card_id=$1)", ids["id"]).Scan(&gone); err != nil || !gone {
		t.Fatalf("account cleanup: %v, %v", gone, err)
	}
}
