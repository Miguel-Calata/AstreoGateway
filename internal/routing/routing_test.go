package routing

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/store"

	"database/sql"
)

var nopLogger = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

type testEnv struct {
	db    *sql.DB
	sel   *Selector
	cache *discovery.Cache
	pool  *keypool.Pool
}

func setup(t *testing.T) *testEnv {
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
	sel := NewSelector(db, cache, pool)
	return &testEnv{db: db, sel: sel, cache: cache, pool: pool}
}

func (e *testEnv) seedProvider(id, name, protocol, baseURL string) {
	e.db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled) VALUES (?, ?, ?, ?, 1)`, id, name, protocol, baseURL)
}

func (e *testEnv) seedKey(providerID, key string) {
	e.db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES (?, ?, 'k', ?, 0, 1)`, "key-"+providerID, providerID, key)
}

func (e *testEnv) seedAlias(name, routing string, targets []model.AliasTarget) {
	e.db.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES (?, ?, ?, 1)`, "alias-"+name, name, routing)
	for _, t := range targets {
		e.db.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES (?, ?, ?, ?)`, "alias-"+name, t.ProviderID, t.ModelName, t.Position)
	}
}

func TestResolveDirectProvider(t *testing.T) {
	e := setup(t)
	e.seedProvider("prov1", "openai", "openai", "http://localhost:9999")
	e.seedKey("prov1", "sk-test")
	e.pool.Load(e.db)

	res, err := e.sel.Resolve("prov1:gpt-5")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Provider.ID != "prov1" {
		t.Fatalf("expected prov1, got %s", res.Provider.ID)
	}
	if res.ModelName != "gpt-5" {
		t.Fatalf("expected gpt-5, got %s", res.ModelName)
	}
	if res.APIKey.Value != "sk-test" {
		t.Fatalf("expected sk-test, got %s", res.APIKey.Value)
	}
	if res.APIKey.ID == "" {
		t.Fatal("expected non-empty key ID")
	}
}

func TestResolveDirectNotFound(t *testing.T) {
	e := setup(t)
	_, err := e.sel.Resolve("noexist:model")
	if err != ErrProviderNotFound {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestResolveDirectAnthropic(t *testing.T) {
	e := setup(t)
	e.seedProvider("anth", "anthropic", "anthropic", "http://localhost:9999")
	e.seedKey("anth", "sk-ant-test")
	e.pool.Load(e.db)

	res, err := e.sel.Resolve("anth:claude-sonnet-4")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Provider.Protocol != "anthropic" {
		t.Fatalf("expected anthropic, got %s", res.Provider.Protocol)
	}
	if res.ModelName != "claude-sonnet-4" {
		t.Fatalf("model: %s", res.ModelName)
	}
}

func TestResolveAliasRoundRobin(t *testing.T) {
	e := setup(t)
	e.seedProvider("p1", "openai", "openai", "http://localhost:9999")
	e.seedProvider("p2", "openai2", "openai", "http://localhost:9998")
	e.seedKey("p1", "sk1")
	e.seedKey("p2", "sk2")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "gpt-4o"}})
	e.seedAlias("coding", "round_robin", []model.AliasTarget{
		{ProviderID: "p1", ModelName: "gpt-5", Position: 0},
		{ProviderID: "p2", ModelName: "gpt-4o", Position: 1},
	})

	r1, err := e.sel.Resolve("coding")
	if err != nil {
		t.Fatalf("resolve 1: %v", err)
	}
	r2, err := e.sel.Resolve("coding")
	if err != nil {
		t.Fatalf("resolve 2: %v", err)
	}
	if r1.ModelName == r2.ModelName {
		t.Fatalf("expected different targets in round_robin, both got %s", r1.ModelName)
	}
}

func TestResolveAliasPriority(t *testing.T) {
	e := setup(t)
	e.seedProvider("p1", "openai", "openai", "http://localhost:9999")
	e.seedProvider("p2", "openai2", "openai", "http://localhost:9998")
	e.seedKey("p1", "sk1")
	e.seedKey("p2", "sk2")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "gpt-4o"}})
	e.seedAlias("fast", "priority", []model.AliasTarget{
		{ProviderID: "p1", ModelName: "gpt-5", Position: 0},
		{ProviderID: "p2", ModelName: "gpt-4o", Position: 1},
	})

	for i := 0; i < 3; i++ {
		r, err := e.sel.Resolve("fast")
		if err != nil {
			t.Fatalf("resolve %d: %v", i, err)
		}
		if r.ModelName != "gpt-5" {
			t.Fatalf("iteration %d: expected gpt-5 (priority), got %s", i, r.ModelName)
		}
	}
}

func TestResolveAliasExcludesStale(t *testing.T) {
	e := setup(t)
	e.seedProvider("p1", "openai", "openai", "http://localhost:9999")
	e.seedProvider("p2", "openai2", "openai", "http://localhost:9998")
	e.seedKey("p1", "sk1")
	e.seedKey("p2", "sk2")
	e.pool.Load(e.db)
	e.seedAlias("coding", "failover", []model.AliasTarget{
		{ProviderID: "p1", ModelName: "gpt-5", Position: 0},
		{ProviderID: "p2", ModelName: "gpt-4o", Position: 1},
	})

	// p1 has no discovered models → gpt-5 is stale
	// p2 has gpt-4o in cache
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "gpt-4o"}})

	r, err := e.sel.Resolve("coding")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.ModelName != "gpt-4o" {
		t.Fatalf("expected gpt-4o (stale gpt-5 excluded), got %s", r.ModelName)
	}
}

func TestResolveAliasAllStale(t *testing.T) {
	e := setup(t)
	e.seedProvider("p1", "openai", "openai", "http://localhost:9999")
	e.seedKey("p1", "sk1")
	e.pool.Load(e.db)
	e.seedAlias("coding", "failover", []model.AliasTarget{
		{ProviderID: "p1", ModelName: "gpt-5", Position: 0},
	})

	// No models injected → all targets stale
	_, err := e.sel.Resolve("coding")
	if err != ErrAliasNoTargets {
		t.Fatalf("expected ErrAliasNoTargets, got %v", err)
	}
}

func TestResolveUnknownModel(t *testing.T) {
	e := setup(t)
	_, err := e.sel.Resolve("nonexistent")
	if err != ErrUnknownModel {
		t.Fatalf("expected ErrUnknownModel, got %v", err)
	}
}

func TestNextFailoverTargetSkipsTried(t *testing.T) {
	e := setup(t)
	e.seedProvider("p1", "openai", "openai", "http://localhost:9999")
	e.seedProvider("p2", "openai2", "openai", "http://localhost:9998")
	e.seedKey("p1", "sk1")
	e.seedKey("p2", "sk2")
	e.pool.Load(e.db)
	e.cache.InjectTestModels("p1", []discovery.Model{{ProviderID: "p1", ModelID: "gpt-5"}})
	e.cache.InjectTestModels("p2", []discovery.Model{{ProviderID: "p2", ModelID: "gpt-4o"}})
	e.seedAlias("coding", "failover", []model.AliasTarget{
		{ProviderID: "p1", ModelName: "gpt-5", Position: 0},
		{ProviderID: "p2", ModelName: "gpt-4o", Position: 1},
	})

	alias, _ := store.GetAliasByName(e.db, "coding")

	tried := map[string]bool{"p1:gpt-5": true}
	target, prov, err := e.sel.NextFailoverTarget(*alias, tried)
	if err != nil {
		t.Fatalf("next failover: %v", err)
	}
	if target.ModelName != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", target.ModelName)
	}
	if prov.ID != "p2" {
		t.Fatalf("expected p2, got %s", prov.ID)
	}
}
