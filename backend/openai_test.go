package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChatVisionProviders(t *testing.T) {
	hosts := map[string]string{"gemini": "generativelanguage.googleapis.com", "groq": "api.groq.com"}
	plan := planLayout("Личная пометка продавца", 1, 2, nil, report{})
	plan.Steps = plan.Steps[:1]
	for _, p := range providers() {
		if _, ok := hosts[p.ID]; !ok {
			continue
		}
		t.Run(p.ID, func(t *testing.T) {
			client := &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != hosts[p.ID] || !strings.HasSuffix(r.URL.Path, "/chat/completions") || r.Header.Get("Authorization") != "Bearer "+p.ID {
					t.Fatal("incorrect endpoint or credentials")
				}
				var request struct {
					Model    string
					Messages []struct {
						Role    string
						Content json.RawMessage
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Model != p.Model || len(request.Messages) != 2 {
					t.Fatal("incorrect model or messages", err)
				}
				var instructions string
				var parts []struct {
					Type, Text string
					Image      struct{ URL string } `json:"image_url"`
				}
				if json.Unmarshal(request.Messages[0].Content, &instructions) != nil || !strings.Contains(instructions, methodForReview()) ||
					json.Unmarshal(request.Messages[1].Content, &parts) != nil || len(parts) != 2 || strings.Contains(parts[0].Text, plan.Title) || !strings.Contains(parts[0].Text, plan.Steps[0].Name) || parts[1].Image.URL != "data:image/png;base64,AQI=" {
					t.Fatal("missing rules, plan or cover")
				}
				body := `{"choices":[{"finish_reason":"stop","message":{"content":"{\"observation\":\"Фон\",\"changes\":[\"Укрупнить товар\"]}"}}]}`
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			got, err := reviewCover(context.Background(), client, p.ID, p, plan, "image/png", []byte{1, 2})
			if err != nil || got.Observation != "Фон" || got.Changes[0] != "Укрупнить товар" {
				t.Fatal(got, err)
			}
		})
	}
}

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReviewCover(t *testing.T) {
	plan := planLayout("Ноутбук", 1, 2, nil, report{})
	plan.Steps = plan.Steps[:1]
	for _, tc := range []struct {
		name, body string
		code       int
		ok         bool
	}{
		{"complete", `{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"observation\":\"Светлый фон\",\"changes\":[\"Увеличить заголовок\"]}"}]}]}`, 200, true},
		{"refused", `{"status":"completed","output":[{"content":[{"type":"refusal"}]}]}`, 200, false},
		{"missing steps", `{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"observation\":\"Фон\",\"changes\":[]}"}]}]}`, 200, false},
		{"quota", "{}", 429, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer test" || r.URL.Path != "/v1/responses" {
					t.Fatal("request setup")
				}
				var data struct {
					Store bool
					Model string
					Input []struct {
						Content []struct {
							Type string
							URL  string `json:"image_url"`
						}
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&data); err != nil || data.Store || data.Model != "test-model" || len(data.Input) != 1 || len(data.Input[0].Content) != 2 || data.Input[0].Content[1].URL != "data:image/png;base64,AQI=" {
					t.Fatal("unexpected request", err)
				}
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			got, err := reviewCover(context.Background(), client, "test", aiProvider{ID: "openai", Model: "test-model", endpoint: "https://api.openai.com/v1/responses"}, plan, "image/png", []byte{1, 2})
			if (err == nil) != tc.ok || (tc.ok && got.Observation != "Светлый фон") {
				t.Fatal(got, err)
			}
		})
	}
}
