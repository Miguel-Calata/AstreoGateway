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

func TestEmbeddingsPassthrough(t *testing.T) {
	var gotPath, gotAuth, upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		upstreamBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []any{map[string]any{"embedding": []float64{0.1, 0.2}, "index": 0}},
		})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL+"/v1", "sk-emb")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "text-embedding-3-small"}})

	body := []byte(`{"model":"prov1:text-embedding-3-small","input":"hello"}`)
	w := httptest.NewRecorder()
	e.proxy.Embeddings(context.Background(), w, e.sel, "prov1:text-embedding-3-small", body)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/embeddings" {
		t.Fatalf("path: got %q", gotPath)
	}
	if gotAuth != "Bearer sk-emb" {
		t.Fatalf("auth: got %q", gotAuth)
	}
	if !strings.Contains(upstreamBody, `"model":"text-embedding-3-small"`) {
		t.Fatalf("model not rewritten: %s", upstreamBody)
	}
	if strings.Contains(upstreamBody, "prov1:") {
		t.Fatalf("provider prefix leaked: %s", upstreamBody)
	}
}

func TestEmbeddingsAnthropicRejected(t *testing.T) {
	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('anth', 'anthropic', 'anthropic', 'https://api.anthropic.com', 1)`)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k-anth', 'anth', 'k', 'sk-ant', 0, 1)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("anth", []discovery.Model{{ProviderID: "anth", ModelID: "claude-sonnet-4"}})

	body := []byte(`{"model":"anth:claude-sonnet-4","input":"hello"}`)
	w := httptest.NewRecorder()
	e.proxy.Embeddings(context.Background(), w, e.sel, "anth:claude-sonnet-4", body)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "protocol does not support embeddings") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestEmbeddingsFailover(t *testing.T) {
	call1 := 0
	upstream1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call1++
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer upstream1.Close()

	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{}})
	}))
	defer upstream2.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p1', 'p1', 'openai', ?, 1)`, upstream1.URL+"/v1")
	e.db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p2', 'p2', 'openai', ?, 1)`, upstream2.URL+"/v1")
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k1', 'p1', 'k', 'sk1', 0, 1)`)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k2', 'p2', 'k', 'sk2', 0, 1)`)
	e.db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES ('a1', 'embed', 'failover', 1)`)
	e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('a1', 'p1', 'emb-1', 0)`)
	e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('a1', 'p2', 'emb-2', 1)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "emb-1"}})
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "emb-2"}})

	body := []byte(`{"model":"embed","input":"x"}`)
	w := httptest.NewRecorder()
	e.proxy.Embeddings(context.Background(), w, e.sel, "embed", body)

	if w.Code != 200 {
		t.Fatalf("expected 200 from failover, got %d: %s", w.Code, w.Body.String())
	}
	if call1 != 1 {
		t.Fatalf("expected 1 call to upstream1, got %d", call1)
	}
}
