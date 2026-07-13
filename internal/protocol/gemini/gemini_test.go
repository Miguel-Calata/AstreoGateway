package gemini

import (
	"encoding/json"
	"testing"
)

func TestBuildModelsURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"https://generativelanguage.googleapis.com", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"https://generativelanguage.googleapis.com/", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"https://generativelanguage.googleapis.com/v1beta", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"https://generativelanguage.googleapis.com/v1beta/", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"http://localhost:8080", "http://localhost:8080/v1beta/models"},
		{"", "/v1beta/models"},
	}
	for _, tt := range tests {
		got := BuildModelsURL(tt.base)
		if got != tt.want {
			t.Errorf("BuildModelsURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestBuildGenerateContentURL(t *testing.T) {
	tests := []struct {
		base  string
		model string
		want  string
	}{
		{"https://generativelanguage.googleapis.com", "gemini-2.5-flash",
			"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"},
		{"https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-flash",
			"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"},
		{"", "gemini-2.5-flash", "/v1beta/models/gemini-2.5-flash:generateContent"},
	}
	for _, tt := range tests {
		got := BuildGenerateContentURL(tt.base, tt.model)
		if got != tt.want {
			t.Errorf("BuildGenerateContentURL(%q, %q) = %q, want %q", tt.base, tt.model, got, tt.want)
		}
	}
}

func TestBuildStreamGenerateContentURL(t *testing.T) {
	got := BuildStreamGenerateContentURL("https://generativelanguage.googleapis.com", "gemini-2.5-flash")
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseModels(t *testing.T) {
	p := &Protocol{}
	body := `{
		"models": [
			{"name": "models/gemini-2.5-flash", "displayName": "Gemini 2.5 Flash", "supportedGenerationMethods": ["generateContent", "countTokens"]},
			{"name": "models/gemini-2.0-pro-exp", "displayName": "Gemini 2.0 Pro Exp", "supportedGenerationMethods": ["generateContent"]},
			{"name": "models/text-embedding-004", "displayName": "Text Embedding 004", "supportedGenerationMethods": ["embedContent"]}
		]
	}`

	entries, err := p.ParseModels([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 models, got %d", len(entries))
	}
	if entries[0].ID != "gemini-2.5-flash" {
		t.Fatalf("first model id: %q", entries[0].ID)
	}
	if entries[0].OwnedBy != "Gemini 2.5 Flash" {
		t.Fatalf("owned_by: %q", entries[0].OwnedBy)
	}
	if entries[1].ID != "gemini-2.0-pro-exp" {
		t.Fatalf("second model id: %q", entries[1].ID)
	}
}

func TestParseModelsEmptyResponse(t *testing.T) {
	p := &Protocol{}
	entries, err := p.ParseModels(json.RawMessage(`{"models":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0, got %d", len(entries))
	}
}

func TestParseModelsNoModelsKey(t *testing.T) {
	p := &Protocol{}
	entries, err := p.ParseModels(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0, got %d", len(entries))
	}
}

func TestParseModelsStripsPrefix(t *testing.T) {
	p := &Protocol{}
	body := `{"models":[{"name":"models/some-vendor/gemini-2.0", "supportedGenerationMethods":["generateContent"]}]}`
	entries, err := p.ParseModels([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "some-vendor/gemini-2.0" {
		t.Fatalf("got %#v", entries)
	}
}

func TestParseModelsInvalidJSON(t *testing.T) {
	p := &Protocol{}
	_, err := p.ParseModels([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error")
	}
}
