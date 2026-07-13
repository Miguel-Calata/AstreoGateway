package routing

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/store"
)

var ErrUnknownModel = errors.New("unknown model")
var ErrAliasNoTargets = errors.New("alias has no available targets")
var ErrProviderNotFound = errors.New("provider not found")
var ErrProtocolMismatch = errors.New("protocol translation not supported yet")
var ErrNoAPIKey = errors.New("no enabled API key")

type Resolved struct {
	Provider      model.Provider
	APIKey        model.APIKey
	ModelName     string
	AliasRouting  string
}

type Selector struct {
	db    *sql.DB
	cache *discovery.Cache
	pool  *keypool.Pool

	mu          sync.Mutex
	rrPositions map[string]int
}

func NewSelector(db *sql.DB, cache *discovery.Cache, pool *keypool.Pool) *Selector {
	return &Selector{
		db:          db,
		cache:       cache,
		pool:        pool,
		rrPositions: make(map[string]int),
	}
}

func (s *Selector) Resolve(directive string) (*Resolved, error) {
	if idx := strings.Index(directive, ":"); idx >= 0 {
		return s.resolveDirect(directive[:idx], directive[idx+1:])
	}
	return s.resolveAlias(directive)
}

func (s *Selector) NextFailoverTarget(alias model.Alias, tried map[string]bool) (*model.AliasTarget, *model.Provider, error) {
	staleSet := s.buildStaleSet(alias.ID)
	provMap, err := s.loadProviderMap()
	if err != nil {
		return nil, nil, err
	}
	for _, t := range alias.Targets {
		key := t.ProviderID + ":" + t.ModelName
		if tried[key] {
			continue
		}
		if staleSet[key] {
			continue
		}
		prov, ok := provMap[t.ProviderID]
		if !ok || !prov.Enabled {
			continue
		}
		return &t, &prov, nil
	}
	return nil, nil, ErrAliasNoTargets
}

func (s *Selector) resolveDirect(providerRef, modelName string) (*Resolved, error) {
	prov, err := store.GetProviderByID(s.db, providerRef)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	if prov == nil {
		prov, err = store.GetProviderBySlug(s.db, providerRef)
		if err != nil {
			return nil, fmt.Errorf("get provider by slug: %w", err)
		}
	}
	if prov == nil {
		// Legacy: name was briefly used as public prefix before slug.
		prov, err = store.GetProviderByName(s.db, providerRef)
		if err != nil {
			return nil, fmt.Errorf("get provider by name: %w", err)
		}
	}
	if prov == nil || !prov.Enabled {
		return nil, ErrProviderNotFound
	}
	if prov.Protocol != "openai" && prov.Protocol != "anthropic" {
		return nil, ErrProtocolMismatch
	}
	apiKey, ok := s.pool.Get(prov.ID)
	if !ok {
		return nil, ErrNoAPIKey
	}
	return &Resolved{
		Provider:  *prov,
		APIKey:    model.APIKey{ID: apiKey.ID, Value: apiKey.Value},
		ModelName: modelName,
	}, nil
}

func (s *Selector) resolveAlias(name string) (*Resolved, error) {
	alias, err := store.GetAliasByName(s.db, name)
	if err != nil {
		return nil, fmt.Errorf("get alias: %w", err)
	}
	if alias == nil || !alias.Enabled {
		return nil, ErrUnknownModel
	}

	staleSet := s.buildStaleSet(alias.ID)
	provMap, err := s.loadProviderMap()
	if err != nil {
		return nil, err
	}

	var active []model.AliasTarget
	for _, t := range alias.Targets {
		key := t.ProviderID + ":" + t.ModelName
		if staleSet[key] {
			continue
		}
		prov, ok := provMap[t.ProviderID]
		if !ok || !prov.Enabled {
			continue
		}
		active = append(active, t)
	}
	if len(active) == 0 {
		return nil, ErrAliasNoTargets
	}

	target := s.selectTarget(alias, active)
	prov := provMap[target.ProviderID]
	apiKey, ok := s.pool.Get(target.ProviderID)
	if !ok {
		return nil, ErrNoAPIKey
	}
	return &Resolved{
		Provider:     prov,
		APIKey:       model.APIKey{ID: apiKey.ID, Value: apiKey.Value},
		ModelName:    target.ModelName,
		AliasRouting: alias.Routing,
	}, nil
}

func (s *Selector) selectTarget(alias *model.Alias, active []model.AliasTarget) model.AliasTarget {
	switch alias.Routing {
	case "round_robin":
		s.mu.Lock()
		pos := s.rrPositions[alias.ID]
		s.rrPositions[alias.ID] = pos + 1
		s.mu.Unlock()
		return active[pos%len(active)]
	case "random":
		return active[rand.Intn(len(active))]
	default:
		return active[0]
	}
}

func (s *Selector) buildStaleSet(aliasID string) map[string]bool {
	stale := s.cache.StaleTargets()
	out := make(map[string]bool, len(stale))
	for _, st := range stale {
		if st.AliasID == aliasID {
			out[st.ProviderID+":"+st.ModelName] = true
		}
	}
	return out
}

func (s *Selector) loadProviderMap() (map[string]model.Provider, error) {
	providers, err := store.ListProviders(s.db)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	m := make(map[string]model.Provider, len(providers))
	for _, p := range providers {
		m[p.ID] = p
	}
	return m, nil
}

func (s *Selector) LookupAlias(name string) (*model.Alias, error) {
	return store.GetAliasByName(s.db, name)
}
