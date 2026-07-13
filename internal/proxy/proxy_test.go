package proxy

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"
)

var nopLogger = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

type proxyEnv struct {
	proxy *Proxy
	sel   *routing.Selector
	cache *discovery.Cache
	pool  *keypool.Pool
	db    *sql.DB
}

func setupProxy(t *testing.T) *proxyEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool := keypool.New()
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	sel := routing.NewSelector(db, cache, pool)
	prox := New(pool, 5*time.Second, 30*time.Second, nopLogger)
	return &proxyEnv{proxy: prox, sel: sel, cache: cache, pool: pool, db: db}
}

func seedProvAndKey(t *testing.T, db *sql.DB, id, baseURL, key string) {
	t.Helper()
	db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES (?, ?, ?, 'openai', ?, 1)`, id, id, id, baseURL)
	db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES (?, ?, 'k', ?, 0, 1)`, "key-"+id, id, key)
}

func TestPassthroughNonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-upstream" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl-1", "choices": []any{}})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL+"/v1", "sk-upstream")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "gpt-5"}})

	body := []byte(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "prov1:gpt-5", body, false)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected X-Request-Id, got %s", w.Header().Get("X-Request-Id"))
	}
}

func TestPassthroughStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL, "sk-upstream")
	e.pool.Load(e.db)

	body := []byte(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "prov1:gpt-5", body, true)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "data:") {
		t.Fatalf("expected SSE data in body, got %s", w.Body.String())
	}
}

func TestPassthroughModelRewrite(t *testing.T) {
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		upstreamBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl-1"})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL, "sk-upstream")
	e.pool.Load(e.db)

	body := []byte(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "prov1:gpt-5", body, false)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(upstreamBody, `"model":"gpt-5"`) {
		t.Fatalf("expected model to be rewritten to gpt-5, got %s", upstreamBody)
	}
	if strings.Contains(upstreamBody, `"model":"prov1:gpt-5"`) {
		t.Fatalf("model should not contain provider prefix, got %s", upstreamBody)
	}
}

func TestPassthrough429MarkCooldown(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL, "sk-upstream")
	e.pool.Load(e.db)

	body := []byte(`{"model":"prov1:gpt-5","messages":[]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "prov1:gpt-5", body, false)

	if w.Code != 429 {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestFailoverRetryOn5xx(t *testing.T) {
	call1 := 0
	upstream1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call1++
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer upstream1.Close()

	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl-2"})
	}))
	defer upstream2.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p1', 'p1', 'p1', 'openai', ?, 1)`, upstream1.URL)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p2', 'p2', 'p2', 'openai', ?, 1)`, upstream2.URL)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k1', 'p1', 'k', 'sk1', 0, 1)`)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k2', 'p2', 'k', 'sk2', 0, 1)`)
	e.db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES ('a1', 'coding', 'failover', 1)`)
	e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('a1', 'p1', 'gpt-5', 0)`)
	e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('a1', 'p2', 'gpt-4o', 1)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "gpt-4o"}})

	body := []byte(`{"model":"coding","messages":[]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "coding", body, false)

	if w.Code != 200 {
		t.Fatalf("expected 200 from failover, got %d: %s", w.Code, w.Body.String())
	}
	if call1 != 1 {
		t.Fatalf("expected 1 call to upstream1, got %d", call1)
	}
}

func TestNoFailoverFor400(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer upstream.Close()

	e := setupProxy(t)
	e.db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p1', 'p1', 'p1', 'openai', ?, 1)`, upstream.URL)
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k1', 'p1', 'k', 'sk1', 0, 1)`)
	e.db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES ('a1', 'coding', 'failover', 1)`)
	e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('a1', 'p1', 'gpt-5', 0)`)
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})

	body := []byte(`{"model":"coding","messages":[]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "coding", body, false)

	if w.Code != 400 {
		t.Fatalf("expected 400 (no failover), got %d: %s", w.Code, w.Body.String())
	}
}

func TestOpenAIBaseURLWithV1(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl-1"})
	}))
	defer upstream.Close()

	e := setupProxy(t)
	seedProvAndKey(t, e.db, "prov1", upstream.URL+"/v1", "sk-upstream")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "gpt-5"}})

	body := []byte(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}]}`)
	w := httptest.NewRecorder()
	e.proxy.ChatCompletions(context.Background(), w, e.sel, "prov1:gpt-5", body, false)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected path /v1/chat/completions, got %q", gotPath)
	}
}
