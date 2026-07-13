package public

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestChatAuthRequired(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/chat/completions", "application/json",
		bytes.NewBufferString(`{"model":"x:y","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestChatPublicE2E(t *testing.T) {
	var gotPath, gotAuth, upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		upstreamBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl-1", "choices": []any{}})
	}))
	defer upstream.Close()

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	token := "aigw_chat_e2e_token"
	if _, err := store.CreateGatewayKey(db, token, "e2e"); err != nil {
		t.Fatalf("gw key: %v", err)
	}
	if err := store.CreateProvider(db, &model.Provider{
		ID: "prov1", Name: "openai", Protocol: "openai",
		BaseURL: upstream.URL + "/v1", Enabled: true, Headers: map[string]string{},
	}); err != nil {
		t.Fatalf("provider: %v", err)
	}
	if err := store.CreateAPIKey(db, &model.APIKey{
		ProviderID: "prov1", Label: "k", Value: "sk-upstream", Enabled: true,
	}); err != nil {
		t.Fatalf("api key: %v", err)
	}

	pool := keypool.New()
	if err := pool.Load(db); err != nil {
		t.Fatalf("load pool: %v", err)
	}
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "gpt-5"}})
	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, nopLogger)
	handler := NewRouter(db, cache, prox, sel, nopLogger)
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) { r.Mount("/", handler) })
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("upstream path %q", gotPath)
	}
	if gotAuth != "Bearer sk-upstream" {
		t.Fatalf("upstream auth %q", gotAuth)
	}
	if !strings.Contains(upstreamBody, `"model":"gpt-5"`) {
		t.Fatalf("expected model rewrite to gpt-5, got %s", upstreamBody)
	}
	if strings.Contains(upstreamBody, `"model":"prov1:gpt-5"`) {
		t.Fatalf("model should not keep provider prefix, got %s", upstreamBody)
	}
}

func TestChatPublicStreamE2E(t *testing.T) {
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

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	token := "aigw_chat_stream_token"
	if _, err := store.CreateGatewayKey(db, token, "e2e"); err != nil {
		t.Fatalf("gw key: %v", err)
	}
	if err := store.CreateProvider(db, &model.Provider{
		ID: "prov1", Name: "openai", Protocol: "openai",
		BaseURL: upstream.URL + "/v1", Enabled: true, Headers: map[string]string{},
	}); err != nil {
		t.Fatalf("provider: %v", err)
	}
	if err := store.CreateAPIKey(db, &model.APIKey{
		ProviderID: "prov1", Label: "k", Value: "sk-upstream", Enabled: true,
	}); err != nil {
		t.Fatalf("api key: %v", err)
	}

	pool := keypool.New()
	if err := pool.Load(db); err != nil {
		t.Fatalf("load pool: %v", err)
	}
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "gpt-5"}})
	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, nopLogger)
	handler := NewRouter(db, cache, prox, sel, nopLogger)
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) { r.Mount("/", handler) })
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"prov1:gpt-5","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "data:") {
		t.Fatalf("expected SSE data, got %s", body)
	}
}

func TestChatMissingModel(t *testing.T) {
	r, _, token := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions",
		bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChatUnknownModel(t *testing.T) {
	r, _, token := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"no-such-alias","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, body)
	}
}
