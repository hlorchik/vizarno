package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestImagePromptKeepsApprovedCopy(t *testing.T) {
	step := layoutStep{Name: "01", Copy: []string{"Тот самый ноутбук для учёбы", "MateBook D16", "Покупай!"}, Purpose: "узнать товар", Composition: "товар в центре", Acceptance: "текст читается"}
	prompt := imagePrompt(step, 1, 3)
	for _, want := range append(step.Copy, "не придумывай комплектацию") {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(want)) {
			t.Fatalf("prompt misses %q", want)
		}
	}
}

func TestWildberriesPromptRemovesPurchaseCTA(t *testing.T) {
	step := layoutStep{Name: "01", Copy: []string{"Тот самый ноутбук для учёбы", "MateBook D16", "Покупай!"}}
	prompt := imagePrompt(step, 1, 8, "wildberries")
	if strings.Contains(prompt, "\nПокупай!\n") {
		t.Fatal("Wildberries prompt must not print a purchase CTA")
	}
	if !strings.Contains(prompt, "Тот самый ноутбук для учёбы") || !strings.Contains(prompt, "призывы купить") {
		t.Fatal("Wildberries prompt lost the useful copy or platform rule")
	}
}

func TestRequestOpenAIImage(t *testing.T) {
	want := []byte("png-result")
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "image-model" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("invalid request: %#v", body)
		}
		response, _ := json.Marshal(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(want)}}})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header)}, nil
	})}
	got, err := requestOpenAIImage(context.Background(), client, "https://api.example.test/images", "secret", "image-model", "image/png", []byte("source"), "prompt")
	if err != nil || string(got) != string(want) {
		t.Fatalf("got %q, %v", got, err)
	}
}
