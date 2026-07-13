package keypool

import (
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"astreoGateway/internal/model"
	"astreoGateway/internal/store"
)

type keyState struct {
	key       model.APIKey
	cooldown  time.Time
}

type Key struct {
	ID    string
	Value string
}

type Pool struct {
	mu   sync.RWMutex
	keys map[string][]keyState // providerID → keys
}

func New() *Pool {
	return &Pool{keys: make(map[string][]keyState)}
}

func (p *Pool) Load(db *sql.DB) error {
	providers, err := store.ListProviders(db)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, prov := range providers {
		keys, err := store.ListAPIKeysByProvider(db, prov.ID)
		if err != nil {
			slog.Error("keypool: load keys", "provider", prov.ID, "err", err)
			continue
		}
		states := make([]keyState, 0, len(keys))
		for _, k := range keys {
			if k.Enabled {
				states = append(states, keyState{key: k})
			}
		}
		p.keys[prov.ID] = states
	}
	return nil
}

func (p *Pool) Get(providerID string) (Key, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	states, ok := p.keys[providerID]
	if !ok || len(states) == 0 {
		return Key{}, false
	}
	now := time.Now()
	for i := range states {
		s := &states[i]
		if !s.key.Enabled {
			continue
		}
		if now.Before(s.cooldown) {
			continue
		}
		return Key{ID: s.key.ID, Value: s.key.Value}, true
	}
	return Key{}, false
}

func (p *Pool) MarkCooldown(providerID, keyID string, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	states, ok := p.keys[providerID]
	if !ok {
		return
	}
	for i := range states {
		if states[i].key.ID == keyID {
			states[i].cooldown = time.Now().Add(d)
			return
		}
	}
}
