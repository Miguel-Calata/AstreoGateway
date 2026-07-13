package discovery

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/store"
)

type Model struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
	OwnedBy    string `json:"owned_by,omitempty"`
}

type StaleTarget struct {
	AliasID    string `json:"alias_id"`
	AliasName  string `json:"alias_name"`
	ProviderID string `json:"provider_id"`
	ModelName  string `json:"model_name"`
}

type providerEntry struct {
	models    []Model
	fetchedAt time.Time
	err       error
}

type ProviderSnapshot struct {
	Models    []Model   `json:"models"`
	FetchedAt time.Time `json:"fetched_at"`
	Error     string    `json:"error,omitempty"`
	Count     int       `json:"count"`
}

type Cache struct {
	mu      sync.RWMutex
	entries map[string]providerEntry
	db      *sql.DB
	pool    *keypool.Pool
	ttl     time.Duration
	timeout time.Duration
	httpC   *http.Client
	logger  *slog.Logger

	ticker *time.Ticker
	cancel context.CancelFunc
}

func New(db *sql.DB, pool *keypool.Pool, ttl, timeout time.Duration, logger *slog.Logger) *Cache {
	return &Cache{
		entries: make(map[string]providerEntry),
		db:      db,
		pool:    pool,
		ttl:     ttl,
		timeout: timeout,
		httpC:   &http.Client{Timeout: timeout},
		logger:  logger,
	}
}

func (c *Cache) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.ticker = time.NewTicker(c.ttl)
	go c.run(ctx)
}

func (c *Cache) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
}

func (c *Cache) run(ctx context.Context) {
	c.refreshAll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.ticker.C:
			c.refreshAll()
		}
	}
}

func (c *Cache) refreshAll() {
	providers, err := store.ListProviders(c.db)
	if err != nil {
		c.logger.Error("discovery: list providers", "err", err)
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		wg.Add(1)
		go func(p model.Provider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := c.refreshProvider(context.Background(), p.ID); err != nil {
				c.logger.Warn("discovery: refresh", "provider", p.ID, "err", err)
			}
		}(p)
	}
	wg.Wait()
}

func (c *Cache) Refresh(ctx context.Context, providerID string) error {
	return c.refreshProvider(ctx, providerID)
}

func (c *Cache) Models() []Model {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []Model
	for _, e := range c.entries {
		out = append(out, e.models...)
	}
	return out
}

func (c *Cache) ModelsFor(providerID string) []Model {
	c.mu.RLock()
	entry, ok := c.entries[providerID]
	c.mu.RUnlock()

	if !ok || len(entry.models) == 0 {
		if err := c.refreshProvider(context.Background(), providerID); err != nil {
			c.logger.Warn("discovery: lazy refresh", "provider", providerID, "err", err)
		}
		c.mu.RLock()
		entry = c.entries[providerID]
		c.mu.RUnlock()
	}
	return entry.models
}

func (c *Cache) Snapshot() map[string]ProviderSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]ProviderSnapshot, len(c.entries))
	for id, e := range c.entries {
		models := e.models
		if models == nil {
			models = []Model{}
		}
		s := ProviderSnapshot{
			Models:    models,
			FetchedAt: e.fetchedAt,
			Count:     len(models),
		}
		if e.err != nil {
			s.Error = e.err.Error()
		}
		out[id] = s
	}
	return out
}

func (c *Cache) Remove(providerID string) {
	c.mu.Lock()
	delete(c.entries, providerID)
	c.mu.Unlock()
}

func (c *Cache) InjectTestModels(providerID string, models []Model) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if models == nil {
		models = []Model{}
	}
	c.entries[providerID] = providerEntry{
		models:    models,
		fetchedAt: time.Now(),
	}
}

func (c *Cache) StaleTargets() []StaleTarget {
	c.mu.RLock()
	known := make(map[string]map[string]bool)
	for pid, e := range c.entries {
		// Only mark stale after a successful discovery (or a prior success still holding models).
		// No entry / never-succeeded error → unknown, not stale.
		if e.fetchedAt.IsZero() && len(e.models) == 0 {
			continue
		}
		m := make(map[string]bool)
		for _, model := range e.models {
			m[model.ModelID] = true
		}
		known[pid] = m
	}
	c.mu.RUnlock()

	aliases, err := store.ListAliases(c.db)
	if err != nil {
		c.logger.Error("discovery: list aliases", "err", err)
		return []StaleTarget{}
	}
	stale := make([]StaleTarget, 0)
	for _, alias := range aliases {
		for _, t := range alias.Targets {
			models, ok := known[t.ProviderID]
			if !ok {
				continue
			}
			if !models[t.ModelName] {
				stale = append(stale, StaleTarget{
					AliasID:    alias.ID,
					AliasName:  alias.Name,
					ProviderID: t.ProviderID,
					ModelName:  t.ModelName,
				})
			}
		}
	}
	return stale
}

func (c *Cache) refreshProvider(ctx context.Context, providerID string) error {
	prov, err := store.GetProviderByID(c.db, providerID)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}
	if prov == nil {
		return fmt.Errorf("provider not found")
	}
	if !prov.Enabled {
		return fmt.Errorf("provider is disabled")
	}

	apiKey, ok := c.pool.Get(providerID)
	if !ok {
		c.mu.Lock()
		prev := c.entries[providerID]
		models := prev.models
		if models == nil {
			models = []Model{}
		}
		c.entries[providerID] = providerEntry{
			models:    models,
			fetchedAt: prev.fetchedAt,
			err:       fmt.Errorf("no enabled API keys"),
		}
		c.mu.Unlock()
		return fmt.Errorf("no enabled API keys for provider %s", providerID)
	}

	models, err := c.fetchModels(ctx, prov, apiKey.Value)
	if err != nil {
		c.mu.Lock()
		prev := c.entries[providerID]
		kept := prev.models
		if kept == nil {
			kept = []Model{}
		}
		c.entries[providerID] = providerEntry{
			models:    kept,
			fetchedAt: prev.fetchedAt,
			err:       err,
		}
		c.mu.Unlock()
		return err
	}

	if models == nil {
		models = []Model{}
	}
	c.mu.Lock()
	c.entries[providerID] = providerEntry{
		models:    models,
		fetchedAt: time.Now(),
	}
	c.mu.Unlock()
	return nil
}

func (c *Cache) fetchModels(ctx context.Context, prov *model.Provider, apiKey string) ([]Model, error) {
	proto := protocol.Get(prov.Protocol)
	if proto == nil {
		return nil, fmt.Errorf("unsupported protocol: %s", prov.Protocol)
	}

	modelsURL := proto.ModelsURL(prov.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	proto.ModelsAuth(req, apiKey)
	for k, v := range prov.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", modelsURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream %s: status %d", modelsURL, resp.StatusCode)
	}

	var body []byte
	if body, err = io.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	entries, err := proto.ParseModels(body)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	out := make([]Model, 0, len(entries))
	for _, m := range entries {
		out = append(out, Model{
			ProviderID: prov.ID,
			ModelID:    m.ID,
			OwnedBy:    m.OwnedBy,
		})
	}
	return out, nil
}

