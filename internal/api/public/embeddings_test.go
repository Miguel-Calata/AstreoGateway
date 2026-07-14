package public

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/proxy"
	_ "astreoGateway/internal/protocol/registry"
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestEmbeddingsAuthRequired(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/embeddings", "application/json",
		bytes.NewBufferString(`{"model":"x:y","input":"hi"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEmbeddingsPublicE2E(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{}})
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
	token := "aigw_embeddings_e2e_token"
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
		ProviderID: "prov1", Label: "k", Value: "sk-test", Enabled: true,
	}); err != nil {
		t.Fatalf("api key: %v", err)
	}

	pool := keypool.New()
	if err := pool.Load(db); err != nil {
		t.Fatalf("load pool: %v", err)
	}
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	cache.InjectTestModels("prov1", []discovery.Model{{ProviderID: "prov1", ModelID: "text-embedding-3-small"}})
	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, nopLogger)
	handler := NewRouter(db, cache, prox, sel, nopLogger, nil)
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) { r.Mount("/", handler) })
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/embeddings",
		bytes.NewBufferString(`{"model":"prov1:text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if gotPath != "/v1/embeddings" {
		t.Fatalf("upstream path %q", gotPath)
	}
}

func TestEmbeddingsMissingModel(t *testing.T) {
	r, _, token := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/embeddings",
		bytes.NewBufferString(`{"input":"hello"}`))
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
