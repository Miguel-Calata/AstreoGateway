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

func TestGeminiNonStream(t *testing.T) {
	var gotAuth, gotPath string
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("x-goog-api-key")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &upstreamBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"role":  "model",
					"parts": []map[string]any{{"text": "Hello from Gemini"}},
				},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]int{
				"promptTokenCount":     3,
				"candidatesTokenCount": 5,
				"totalTokenCount":      8,
			},
		})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('gem', 'gemini', 'gemini', 'gemini', ?, 1)`, upstream.URL)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('kg', 'gem', 'k', 'goog-key', 0, 1)`)
	e.pool.Load(e.db)

	body := []byte(`{"model":"gem:gemini-2.5-flash","messages":[{"role":"user","content":"hi"}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "gem:gemini-2.5-flash", body, false)

	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if gotAuth != "goog-key" {
		t.Fatalf("auth: %q", gotAuth)
	}
	if !strings.Contains(gotPath, "gemini-2.5-flash:generateContent") {
		t.Fatalf("path: %q", gotPath)
	}
	contents, _ := upstreamBody["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("upstream body: %+v", upstreamBody)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	choices, _ := resp["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("response: %s", w.Body.String())
	}
}

func TestGeminiStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("expected alt=sse, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hi\"}],\"role\":\"model\"}}]}\n\n")
		io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"!\"}],\"role\":\"model\"},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer upstream.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('gem', 'gemini', 'gemini', 'gemini', ?, 1)`, upstream.URL)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('kg', 'gem', 'k', 'goog-key', 0, 1)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("gem", []discovery.Model{{ProviderID: "gem", ModelID: "gemini-2.5-flash"}})

	body := []byte(`{"model":"gem:gemini-2.5-flash","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "gem:gemini-2.5-flash", body, true)

	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	out := w.Body.String()
	if !strings.Contains(out, `"content":"Hi"`) {
		t.Fatalf("missing delta: %s", out)
	}
	if !strings.Contains(out, "data: [DONE]") {
		t.Fatalf("missing DONE: %s", out)
	}
}

func TestEmbeddingsGeminiRejected(t *testing.T) {
	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('gem', 'gemini', 'gemini', 'gemini', 'http://localhost', 1)`)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('kg', 'gem', 'k', 'goog-key', 0, 1)`)
	e.pool.Load(e.db)

	body := []byte(`{"model":"gem:text-embedding","input":"hi"}`)
	w := httptest.NewRecorder()
	e.proxy.Embeddings(context.Background(), w, e.sel, "gem:text-embedding", body)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "embeddings") {
		t.Fatalf("body: %s", w.Body.String())
	}
}
