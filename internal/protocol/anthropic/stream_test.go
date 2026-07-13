package anthropic

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslateStreamText(t *testing.T) {
	sse := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_abc","usage":{"input_tokens":5,"output_tokens":0}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	w := httptest.NewRecorder()
	err := TranslateStream(strings.NewReader(sse), w, "claude", false, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"content":"Hello"`) {
		t.Fatalf("missing content delta: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("missing DONE: %s", body)
	}
	if !strings.Contains(body, "chatcmpl-msg_abc") {
		t.Fatalf("missing id: %s", body)
	}
}

func TestTranslateStreamPingIgnored(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"ping"}`,
		``,
		`data: {"type":"message_start","message":{"id":"m1","usage":{"input_tokens":1,"output_tokens":0}}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
	w := httptest.NewRecorder()
	if err := TranslateStream(strings.NewReader(sse), w, "claude", false, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Body.String(), "[DONE]") {
		t.Fatal(w.Body.String())
	}
}
