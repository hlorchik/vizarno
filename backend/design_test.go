package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReviewExcludesImaginarySource(t *testing.T) {
	plan := planLayout("Draft", 1, 1, nil, report{})
	plan.Steps = plan.Steps[:1]
	plan.Materials = &materials{}
	for _, keep := range []bool{true, false} {
		findings := []insight{{"Ясность", "Слайд 2", "Подпись", "Найти модель", "Добавить", "Прочитать"}}
		if keep {
			findings = append(findings, insight{"Масштаб", "Обложка", "Ноутбук в центре", "Узнать товар", "Укрупнить", "Проверить контур"})
		}
		answer, _ := json.Marshal(visualReview{Observation: "Ноутбук в центре", Changes: []string{"Укрупнить фото"}, Insights: findings, Designs: []slideDesign{{Layout: "hero", Theme: "light", Photo: 0}}})
		response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(answer)}}}})
		client := &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(response)))}, nil
		})}
		got, err := reviewCover(context.Background(), client, "test", aiProvider{ID: "groq", Label: "Groq", endpoint: "https://api.example.test"}, plan, "image/png", []byte{1})
		if !keep {
			if err == nil {
				t.Fatal("accepted ungrounded analysis")
			}
			continue
		}
		if err != nil || got.OmittedInsights != 1 || len(got.Insights) != 1 || got.Insights[0].Source != "Обложка" {
			t.Fatal(got, err)
		}
	}
}

func TestExecutableDesign(t *testing.T) {
	for _, tc := range []struct {
		name   string
		design []slideDesign
		ok     bool
	}{
		{"valid", []slideDesign{{"hero", "light", 0, nil}, {"split", "dark", 1, nil}}, true},
		{"missing", []slideDesign{{"hero", "light", 0, nil}}, false},
		{"imaginary photo", []slideDesign{{"hero", "light", 2, nil}, {"split", "dark", 0, nil}}, false},
		{"negative photo", []slideDesign{{"hero", "light", -1, nil}, {"split", "dark", 0, nil}}, false},
		{"unknown layout", []slideDesign{{"floating", "light", 0, nil}, {"split", "dark", 0, nil}}, false},
		{"unknown palette", []slideDesign{{"hero", "invisible", 0, nil}, {"split", "dark", 0, nil}}, false},
		{"crop outside", []slideDesign{{"hero", "light", 0, &photoCrop{90, 0, 30, 50}}, {"split", "dark", 0, nil}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if validDesigns(tc.design, 2, 1) != tc.ok {
				t.Fatal(tc.design)
			}
		})
	}
}
