package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"astreoGateway/internal/discovery"
)

func TestAnthropicNonStream(t *testing.T) {
	var gotAuth, gotVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" && !strings.HasSuffix(r.URL.Path, "/messages") {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(b, &req)
		if req["model"] != "claude-sonnet-4" {
			t.Errorf("upstream model: %v", req["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_xyz",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "Hello from Claude"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 3, "output_tokens": 5},
		})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('anth', 'anthropic', 'anthropic', 'anthropic', ?, 1)`, upstream.URL+"/v1")
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('ka', 'anth', 'k', 'sk-ant-test', 0, 1)`)
	e.pool.Load(e.db)

	body := []byte(`{"model":"anth:claude-sonnet-4","messages":[{"role":"user","content":"hi"}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "anth:claude-sonnet-4", body, false)

	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if gotAuth != "sk-ant-test" {
		t.Fatalf("x-api-key: %q", gotAuth)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version: %q", gotVersion)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "chatcmpl-msg_xyz" {
		t.Fatalf("id: %v", resp["id"])
	}
	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Hello from Claude" {
		t.Fatalf("content: %v", msg["content"])
	}
}

func TestAnthropicStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_s\",\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
		io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n")
		io.WriteString(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n")
		io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('anth', 'anthropic', 'anthropic', 'anthropic', ?, 1)`, upstream.URL+"/v1")
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('ka', 'anth', 'k', 'sk-ant', 0, 1)`)
	e.pool.Load(e.db)

	body := []byte(`{"model":"anth:claude","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "anth:claude", body, true)

	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"content":"Hi"`) {
		t.Fatalf("body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "[DONE]") {
		t.Fatalf("missing DONE")
	}
}

func TestAnthropicImageRejected(t *testing.T) {
	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('anth', 'anthropic', 'anthropic', 'anthropic', 'http://unused/v1', 1)`)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('ka', 'anth', 'k', 'sk', 0, 1)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("anth", []discovery.Model{{ProviderID: "anth", ModelID: "claude"}})

	body := []byte(`{"model":"anth:claude","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"http://x"}}]}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "anth:claude", body, false)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
