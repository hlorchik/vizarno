package main

import (
	"net/http"
	"os"
)

type aiProvider struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
	env      string
	endpoint string
}

func providers() []aiProvider {
	list := []aiProvider{
		{ID: "gemini", Label: "Google Gemini", Model: "gemini-3.1-flash-lite", env: "GEMINI_API_KEY", endpoint: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"},
		{ID: "groq", Label: "Groq", Model: "qwen/qwen3.8-27b", env: "GROQ_API_KEY", endpoint: "https://api.groq.com/openai/v1/chat/completions"},
		{ID: "openai", Label: "OpenAI", Model: "gpt-4.1-mini", env: "OPENAI_API_KEY", endpoint: "https://api.openai.com/v1/responses"},
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		list[2].Model = model
	}
	for i := range list {
		list[i].Enabled = os.Getenv(list[i].env) != ""
	}
	return list
}
func providerOptions(w http.ResponseWriter, r *http.Request, uid int64) { reply(w, 200, providers()) }
