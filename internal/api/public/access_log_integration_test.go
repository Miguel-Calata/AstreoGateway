package public

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
	"astreoGateway/internal/metrics"
	"astreoGateway/internal/proxy"
	_ "astreoGateway/internal/protocol/registry"
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func setupPublicWithLogs(t *testing.T) (*sql.DB, *metrics.LogStore, http.Handler, *keypool.Pool, *discovery.Cache) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	pool := keypool.New()
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, logger)
	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, logger)
	prox.SetDiscoveryCache(cache)
	logs := metrics.NewLogStore(100)
	handler := NewRouter(db, cache, prox, sel, logger, logs)
	r := chi.NewRouter()
	r.Mount("/", handler)
	return db, logs, r, pool, cache
}

func seedGatewayKey(t *testing.T, db *sql.DB) (token, id string) {
	t.Helper()
	token = "aigw_testtoken_integration_001"
	k, err := store.CreateGatewayKey(db, token, "test")
	if err != nil {
		t.Fatal(err)
	}
	return token, k.ID
}

func TestAccessLogRecordsGatewayKeyAndTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-1",
			"choices": []any{
				map[string]any{"message": map[string]any{"role": "assistant", "content": "hi"}},
			},
			"usage": map[string]any{"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13},
		})
	}))
	defer upstream.Close()

	db, logs, handler, pool, cache := setupPublicWithLogs(t)
	token, keyID := seedGatewayKey(t, db)
	db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p1','P','oa','openai',?,1)`, upstream.URL+"/v1")
	db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k1','p1','k','sk',0,1)`)
	pool.Load(db)
	cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})

	body := `{"model":"oa:gpt-5","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	res := logs.List(metrics.ListQuery{Limit: 10})
	if res.Total != 1 {
		t.Fatalf("logs=%d", res.Total)
	}
	e := res.Items[0]
	if e.GatewayKeyID != keyID {
		t.Fatalf("gateway_key_id=%q want %q", e.GatewayKeyID, keyID)
	}
	if e.Directive != "oa:gpt-5" {
		t.Fatalf("directive=%q", e.Directive)
	}
	if e.ResolvedProvider != "oa" || e.ResolvedModel != "gpt-5" {
		t.Fatalf("resolved=%s:%s", e.ResolvedProvider, e.ResolvedModel)
	}
	if e.TokensPrompt != 9 || e.TokensCompletion != 4 {
		t.Fatalf("tokens=%d/%d", e.TokensPrompt, e.TokensCompletion)
	}
	if e.Status != 200 {
		t.Fatalf("status=%d", e.Status)
	}
	if e.ID == "" || e.RequestID == "" || e.ID != e.RequestID {
		t.Fatalf("id mismatch: id=%q rid=%q", e.ID, e.RequestID)
	}
	if e.Attempts != 1 || len(e.AttemptsDetail) != 1 {
		t.Fatalf("attempts=%d detail=%d", e.Attempts, len(e.AttemptsDetail))
	}
}

func TestAccessLogStreamTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// verify include_usage was injected
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "include_usage") {
			t.Errorf("expected include_usage in body: %s", b)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1}}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	db, logs, handler, pool, cache := setupPublicWithLogs(t)
	token, _ := seedGatewayKey(t, db)
	db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p1','P','oa','openai',?,1)`, upstream.URL+"/v1")
	db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('k1','p1','k','sk',0,1)`)
	pool.Load(db)
	cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})

	body := `{"model":"oa:gpt-5","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d", w.Code)
	}

	res := logs.List(metrics.ListQuery{Limit: 5})
	if res.Total != 1 {
		t.Fatalf("logs=%d", res.Total)
	}
	e := res.Items[0]
	if e.TokensPrompt != 2 || e.TokensCompletion != 1 {
		t.Fatalf("stream tokens=%d/%d", e.TokensPrompt, e.TokensCompletion)
	}
	if !e.Stream {
		t.Fatal("expected stream=true")
	}
	if e.Status != 200 {
		t.Fatalf("status=%d", e.Status)
	}
}

func TestAccessLogFailoverAttempts(t *testing.T) {
	var hits int
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, `{"error":{"message":"down"}}`, http.StatusBadGateway)
	}))
	defer upA.Close()
	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		json.NewEncoder(w).Encode(map[string]any{
			"id": "ok",
			"choices": []any{},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer upB.Close()

	db, logs, handler, pool, cache := setupPublicWithLogs(t)
	token, _ := seedGatewayKey(t, db)
	db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('pa','A','a','openai',?,1)`, upA.URL+"/v1")
	db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('pb','B','b','openai',?,1)`, upB.URL+"/v1")
	db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('ka','pa','k','sk',0,1)`)
	db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES ('kb','pb','k','sk',0,1)`)
	db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES ('al1','coding','failover',1)`)
	db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES ('al1','pa','m',0),('al1','pb','m',1)`)
	pool.Load(db)
	cache.InjectTestModels("pa", []discovery.Model{{ProviderID: "pa", ModelID: "m"}})
	cache.InjectTestModels("pb", []discovery.Model{{ProviderID: "pb", ModelID: "m"}})

	body := `{"model":"coding","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if hits < 2 {
		t.Fatalf("expected failover hits>=2, got %d", hits)
	}

	res := logs.List(metrics.ListQuery{Limit: 5})
	e := res.Items[0]
	if e.AliasName != "coding" {
		t.Fatalf("alias=%q", e.AliasName)
	}
	if len(e.AttemptsDetail) < 2 {
		t.Fatalf("attempts_detail=%+v", e.AttemptsDetail)
	}
	if e.ResolvedProvider != "b" {
		t.Fatalf("resolved provider=%q", e.ResolvedProvider)
	}
	if e.Status != 200 {
		t.Fatalf("status=%d", e.Status)
	}
}

// silence unused import if routing context ever needed
var _ = context.Background
