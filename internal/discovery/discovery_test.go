package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/store"
)

var nopLogger = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

func testCache(t *testing.T) (*Cache, *keypool.Pool, func()) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool := keypool.New()
	cache := New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	return cache, pool, func() { db.Close() }
}

func seedProvider(t *testing.T, db *sql.DB, id, name, protocol, baseURL string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES (?, ?, ?, ?, ?, 1)`, id, name, store.Slugify(name), protocol, baseURL)
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
}

func seedKey(t *testing.T, db *sql.DB, providerID, key string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES (?, ?, 'test', ?, 0, 1)`, "key-"+providerID, providerID, key)
	if err != nil {
		t.Fatalf("seed key: %v", err)
	}
}

func seedAlias(t *testing.T, db *sql.DB, aliasID, name, routing string, targets []model.AliasTarget) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES (?, ?, ?, 1)`, aliasID, name, routing)
	if err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	for _, tgt := range targets {
		_, err = db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES (?, ?, ?, ?)`, aliasID, tgt.ProviderID, tgt.ModelName, tgt.Position)
		if err != nil {
			t.Fatalf("seed target: %v", err)
		}
	}
}

func mockModelsServer(t *testing.T, models []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		entries := make([]struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by,omitempty"`
		}, len(models))
		for i, m := range models {
			entries[i] = struct {
				ID      string `json:"id"`
				OwnedBy string `json:"owned_by,omitempty"`
			}{ID: m, OwnedBy: "provider"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": entries})
	}))
}

func TestCacheRefresh(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	ts := mockModelsServer(t, []string{"gpt-5", "gpt-4o"})
	defer ts.Close()

	seedProvider(t, db, "prov1", "openai", "openai", ts.URL)
	seedKey(t, db, "prov1", "sk-test123456789")
	pool.Load(db)

	if err := cache.refreshProvider(context.Background(), "prov1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	models := cache.Models()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	ids := map[string]bool{}
	for _, m := range models {
		ids[m.ModelID] = true
	}
	if !ids["gpt-5"] || !ids["gpt-4o"] {
		t.Fatalf("unexpected models: %v", models)
	}
}

func TestStaleTargetsDetection(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	ts := mockModelsServer(t, []string{"gpt-5"})
	defer ts.Close()

	seedProvider(t, db, "prov1", "openai", "openai", ts.URL)
	seedKey(t, db, "prov1", "sk-test123456789")
	pool.Load(db)

	seedAlias(t, db, "alias-1", "coding", "failover", []model.AliasTarget{
		{ProviderID: "prov1", ModelName: "gpt-5", Position: 0},
		{ProviderID: "prov1", ModelName: "gpt-4o", Position: 1},
	})

	if err := cache.refreshProvider(context.Background(), "prov1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	stale := cache.StaleTargets()
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale target, got %d", len(stale))
	}
	if stale[0].ModelName != "gpt-4o" {
		t.Fatalf("expected stale gpt-4o, got %s", stale[0].ModelName)
	}
	if stale[0].AliasName != "coding" {
		t.Fatalf("expected alias coding, got %s", stale[0].AliasName)
	}
}

func TestCacheKeepsOldOnError(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			entries := []struct {
				ID string `json:"id"`
			}{{ID: "gpt-5"}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": entries})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	seedProvider(t, db, "prov1", "openai", "openai", ts.URL)
	seedKey(t, db, "prov1", "sk-test123456789")
	pool.Load(db)

	if err := cache.refreshProvider(context.Background(), "prov1"); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if err := cache.refreshProvider(context.Background(), "prov1"); err == nil {
		t.Fatal("expected error on second refresh")
	}

	models := cache.Models()
	if len(models) != 1 {
		t.Fatalf("expected 1 model preserved, got %d", len(models))
	}
	if models[0].ModelID != "gpt-5" {
		t.Fatalf("expected preserved gpt-5, got %s", models[0].ModelID)
	}
}

func TestModelsForLazyRefresh(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	ts := mockModelsServer(t, []string{"gpt-5"})
	defer ts.Close()

	seedProvider(t, db, "prov1", "openai", "openai", ts.URL)
	seedKey(t, db, "prov1", "sk-test123456789")
	pool.Load(db)

	models := cache.ModelsFor("prov1")
	if len(models) != 1 {
		t.Fatalf("expected 1 model after lazy refresh, got %d", len(models))
	}
	if models[0].ModelID != "gpt-5" {
		t.Fatalf("expected gpt-5, got %s", models[0].ModelID)
	}
}

func TestAnthropicProtocolUsesXAPIKey(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	var gotHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-api-key")
		entries := []struct{ ID string }{{ID: "claude-sonnet-4"}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": entries})
	}))
	defer ts.Close()

	seedProvider(t, db, "anth", "anthropic", "anthropic", ts.URL)
	seedKey(t, db, "anth", "sk-ant-test123456789")
	pool.Load(db)

	if err := cache.refreshProvider(context.Background(), "anth"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if gotHeader != "sk-ant-test123456789" {
		t.Fatalf("expected x-api-key header, got %q", gotHeader)
	}
}

func TestBuildModelsURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"https://api.openai.com/v1", "https://api.openai.com/v1/models"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/models"},
		{"http://localhost:8080", "http://localhost:8080/models"},
		{"", "/models"},
	}
	for _, tt := range tests {
		got := buildModelsURL(tt.base)
		if got != tt.want {
			t.Errorf("buildModelsURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestNoKeysPreservesEntry(t *testing.T) {
	cache, pool, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	seedProvider(t, db, "prov1", "openai", "openai", "http://unused")
	pool.Load(db)

	err := cache.refreshProvider(context.Background(), "prov1")
	if err == nil {
		t.Fatal("expected error for provider with no keys")
	}
	if err.Error() != "no enabled API keys for provider prov1" {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := cache.Snapshot()["prov1"]
	if snap.Models == nil {
		t.Fatal("expected non-nil models slice in snapshot")
	}
	if len(snap.Models) != 0 {
		t.Fatalf("expected empty models, got %d", len(snap.Models))
	}
	if snap.Error == "" {
		t.Fatal("expected error string in snapshot")
	}
}

func TestStaleRequiresSuccessfulDiscovery(t *testing.T) {
	cache, _, cleanup := testCache(t)
	defer cleanup()
	db := cache.db

	seedProvider(t, db, "prov1", "openai", "openai", "http://unused")
	seedAlias(t, db, "alias-1", "coding", "failover", []model.AliasTarget{
		{ProviderID: "prov1", ModelName: "gpt-5", Position: 0},
	})

	// No discovery yet → not stale
	if stale := cache.StaleTargets(); len(stale) != 0 {
		t.Fatalf("expected 0 stale before discovery, got %d", len(stale))
	}

	// Failed first refresh (no keys) → still not stale
	_ = cache.refreshProvider(context.Background(), "prov1")
	if stale := cache.StaleTargets(); len(stale) != 0 {
		t.Fatalf("expected 0 stale after failed discovery, got %d", len(stale))
	}

	// Successful discovery without gpt-5 → stale
	cache.InjectTestModels("prov1", []Model{{ProviderID: "prov1", ModelID: "other"}})
	stale := cache.StaleTargets()
	if len(stale) != 1 || stale[0].ModelName != "gpt-5" {
		t.Fatalf("expected gpt-5 stale after successful discovery, got %#v", stale)
	}
}

func TestRemoveClearsEntry(t *testing.T) {
	cache, _, cleanup := testCache(t)
	defer cleanup()
	cache.InjectTestModels("prov1", []Model{{ProviderID: "prov1", ModelID: "gpt-5"}})
	cache.Remove("prov1")
	if _, ok := cache.Snapshot()["prov1"]; ok {
		t.Fatal("expected entry removed from snapshot")
	}
}


