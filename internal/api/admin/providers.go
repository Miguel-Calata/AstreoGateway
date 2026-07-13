package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/model"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func validateProviderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "name is required"
	}
	if strings.Contains(name, ":") {
		return "name must not contain ':'"
	}
	return ""
}

func validateProviderSlug(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if !store.ValidSlug(store.Slugify(slug)) && !store.ValidSlug(slug) {
		return "slug must be lowercase letters, digits, '-', '_' or '.' (no spaces or ':')"
	}
	return ""
}

func providersRouter(db *sql.DB, cache *discovery.Cache) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listProviders(db))
	r.Post("/", createProvider(db))
	r.Route("/{providerID}", func(r chi.Router) {
		r.Get("/", getProvider(db))
		r.Put("/", updateProvider(db))
		r.Delete("/", deleteProvider(db, cache))
	})
	return r
}

func listProviders(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providers, err := store.ListProviders(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, providers)
	}
}

func createProvider(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p model.Provider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		p.Name = strings.TrimSpace(p.Name)
		p.Slug = strings.TrimSpace(p.Slug)
		if p.Name == "" || p.Protocol == "" || p.BaseURL == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "name, protocol, and base_url are required"})
			return
		}
		if msg := validateProviderName(p.Name); msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		if msg := validateProviderSlug(p.Slug); msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		if p.Protocol != "openai" && p.Protocol != "anthropic" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "protocol must be openai or anthropic"})
			return
		}
		if p.Headers == nil {
			p.Headers = map[string]string{}
		}
		if err := store.CreateProvider(db, &p); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, p)
	}
}

func getProvider(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "providerID")
		p, err := store.GetProviderByID(db, id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		if p == nil {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"error": "provider not found"})
			return
		}
		writeJSON(w, p)
	}
}

func updateProvider(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "providerID")
		var p model.Provider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		p.ID = id
		p.Name = strings.TrimSpace(p.Name)
		p.Slug = strings.TrimSpace(p.Slug)
		if msg := validateProviderName(p.Name); msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		if msg := validateProviderSlug(p.Slug); msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		if p.Headers == nil {
			p.Headers = map[string]string{}
		}
		if err := store.UpdateProvider(db, &p); err != nil {
			if err.Error() == "provider not found" {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, map[string]any{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, p)
	}
}

func deleteProvider(db *sql.DB, cache *discovery.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "providerID")
		if err := store.DeleteProvider(db, id); err != nil {
			if err.Error() == "provider not found" {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, map[string]any{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		if cache != nil {
			cache.Remove(id)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
