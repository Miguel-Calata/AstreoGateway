package openai

import "testing"

func TestBuildChatCompletionsURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/chat/completions"},
		{"http://localhost:8080", "http://localhost:8080/chat/completions"},
		{"", "/chat/completions"},
	}
	for _, tt := range tests {
		got := BuildChatCompletionsURL(tt.base)
		if got != tt.want {
			t.Errorf("BuildChatCompletionsURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestBuildEmbeddingsURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"https://api.openai.com/v1", "https://api.openai.com/v1/embeddings"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/embeddings"},
		{"http://localhost:8080", "http://localhost:8080/embeddings"},
		{"", "/embeddings"},
	}
	for _, tt := range tests {
		got := BuildEmbeddingsURL(tt.base)
		if got != tt.want {
			t.Errorf("BuildEmbeddingsURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}
