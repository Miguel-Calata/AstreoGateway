package keypool

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"astreoGateway/internal/model"
	"astreoGateway/internal/store"
)

func testPoolDB(t *testing.T) (*Pool, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(), db
}

func TestLoadGetEnabled(t *testing.T) {
	pool, db := testPoolDB(t)
	if _, err := db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p1', 'p1', 'openai', 'https://api.example.com/v1', 1)`); err != nil {
		t.Fatalf("insert provider: %v", err)
	}
	k := &model.APIKey{ProviderID: "p1", Label: "main", Value: "sk-test", Priority: 0, Enabled: true}
	if err := store.CreateAPIKey(db, k); err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := pool.Load(db); err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := pool.Get("p1")
	if !ok {
		t.Fatal("expected key")
	}
	if got.Value != "sk-test" || got.ID != k.ID {
		t.Fatalf("got %+v", got)
	}
}

func TestGetSkipsDisabled(t *testing.T) {
	pool, db := testPoolDB(t)
	db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p1', 'p1', 'openai', 'https://x', 1)`)
	k := &model.APIKey{ProviderID: "p1", Label: "off", Value: "sk-off", Enabled: false}
	store.CreateAPIKey(db, k)
	pool.Load(db)
	if _, ok := pool.Get("p1"); ok {
		t.Fatal("disabled key should not be returned")
	}
}

func TestMarkCooldown(t *testing.T) {
	pool, db := testPoolDB(t)
	db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p1', 'p1', 'openai', 'https://x', 1)`)
	k := &model.APIKey{ProviderID: "p1", Label: "main", Value: "sk-test", Enabled: true}
	store.CreateAPIKey(db, k)
	pool.Load(db)

	pool.MarkCooldown("p1", k.ID, time.Hour)
	if _, ok := pool.Get("p1"); ok {
		t.Fatal("key in cooldown should not be returned")
	}
}

func TestLoadReloadPicksNewKey(t *testing.T) {
	pool, db := testPoolDB(t)
	db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES ('p1', 'p1', 'openai', 'https://x', 1)`)
	pool.Load(db)
	if _, ok := pool.Get("p1"); ok {
		t.Fatal("expected no keys")
	}
	k := &model.APIKey{ProviderID: "p1", Label: "new", Value: "sk-new", Enabled: true}
	store.CreateAPIKey(db, k)
	pool.Load(db)
	got, ok := pool.Get("p1")
	if !ok || got.Value != "sk-new" {
		t.Fatalf("expected sk-new after reload, got %+v ok=%v", got, ok)
	}
}

func TestGetUnknownProvider(t *testing.T) {
	pool := New()
	if _, ok := pool.Get("missing"); ok {
		t.Fatal("expected false")
	}
}
