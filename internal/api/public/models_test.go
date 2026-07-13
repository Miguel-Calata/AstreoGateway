package public

import (
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
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"

	"log/slog"

	"github.com/go-chi/chi/v5"
)

var nopLogger = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

func testSetup(t *testing.T) (*chi.Mux, *discovery.Cache, string) {
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
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, nopLogger)
	publicHandler := NewRouter(db, cache, prox, sel, nopLogger)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Mount("/", publicHandler)
	})

	gwKey, err := store.CreateGatewayKey(db, "aigw_testtoken1234567890", "test-key")
	if err != nil {
		t.Fatalf("create gw key: %v", err)
	}
	_ = gwKey

	return r, cache, "aigw_testtoken1234567890"
}

func TestListModelsAuthRequired(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListModelsReturnsModelsAndAliases(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	_, err = store.CreateGatewayKey(db, "aigw_testtoken1234567890", "test-key")
	if err != nil {
		t.Fatalf("create gw key: %v", err)
	}

	err = store.CreateProvider(db, &model.Provider{
		ID:       "prov1",
		Name:     "openai",
		Protocol: "openai",
		BaseURL:  "http://unused",
		Enabled:  true,
		Headers:  map[string]string{},
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	err = store.CreateAlias(db, &model.Alias{
		Name:    "coding",
		Routing: "failover",
		Enabled: true,
		Targets: []model.AliasTarget{
			{ProviderID: "prov1", ModelName: "gpt-5", Position: 0},
		},
	})
	if err != nil {
		t.Fatalf("create alias: %v", err)
	}

	pool := keypool.New()
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, 5*time.Second, 30*time.Second, nopLogger)
	publicHandler := NewRouter(db, cache, prox, sel, nopLogger)
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Mount("/", publicHandler)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer aigw_testtoken1234567890")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body modelsResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Object != "list" {
		t.Fatalf("expected object=list, got %s", body.Object)
	}

	found := false
	for _, m := range body.Data {
		if m.ID == "coding" && m.OwnedBy == "alias" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected alias 'coding' in response, got %v", body.Data)
	}
}

func TestListModelsInvalidKey(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer aigw_invalidkey1234567890")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
