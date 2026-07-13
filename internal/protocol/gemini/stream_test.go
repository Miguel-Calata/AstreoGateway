package gemini

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslateStreamText(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`,
		``,
		`data: {"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
		``,
	}, "\n")

	w := httptest.NewRecorder()
	err := TranslateStream(strings.NewReader(sse), w, "gemini-2.5-flash", true, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"content":"Hello"`) {
		t.Fatalf("missing content delta: %s", body)
	}
	if !strings.Contains(body, `"content":" world"`) {
		t.Fatalf("missing second delta: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("missing DONE: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("missing finish: %s", body)
	}
	if !strings.Contains(body, `"prompt_tokens":5`) {
		t.Fatalf("missing usage: %s", body)
	}
}

func TestTranslateStreamFunctionCall(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"city":"NYC"}}}],"role":"model"},"finishReason":"STOP"}]}`,
		``,
	}, "\n")
	w := httptest.NewRecorder()
	if err := TranslateStream(strings.NewReader(sse), w, "gemini", false, nil); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"name":"get_weather"`) {
		t.Fatalf("missing tool: %s", body)
	}
	if !strings.Contains(body, "tool_calls") {
		t.Fatalf("missing tool_calls finish: %s", body)
	}
}
