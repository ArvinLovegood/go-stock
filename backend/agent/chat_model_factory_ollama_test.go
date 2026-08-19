package agent

import (
	"testing"
)

func TestNormalizeOllamaBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "http://127.0.0.1:11434"},
		{"plain", "http://127.0.0.1:11434", "http://127.0.0.1:11434"},
		{"trailing slash", "http://127.0.0.1:11434/", "http://127.0.0.1:11434"},
		{"v1 suffix", "http://127.0.0.1:11434/v1", "http://127.0.0.1:11434"},
		{"v1 trailing slash", "http://127.0.0.1:11434/v1/", "http://127.0.0.1:11434"},
		{"v1 chat completions", "http://127.0.0.1:11434/v1/chat/completions", "http://127.0.0.1:11434"},
		{"v1 chat completions trailing slash", "http://127.0.0.1:11434/v1/chat/completions/", "http://127.0.0.1:11434"},
		{"reverse proxy prefix preserved", "http://example.com/ollama", "http://example.com/ollama"},
		{"reverse proxy with v1 stripped", "http://example.com/ollama/v1", "http://example.com/ollama"},
		{"https", "https://ollama.example.com:11434/v1", "https://ollama.example.com:11434"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOllamaBaseURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeOllamaBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
