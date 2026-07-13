package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"astreoGateway/internal/model"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func providersRouter(db *sql.DB) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listProviders(db))
	r.Post("/", createProvider(db))
	r.Route("/{providerID}", func(r chi.Router) {
		r.Get("/", getProvider(db))
		r.Put("/", updateProvider(db))
		r.Delete("/", deleteProvider(db))
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
		if p.Name == "" || p.Protocol == "" || p.BaseURL == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "name, protocol, and base_url are required"})
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

func deleteProvider(db *sql.DB) http.HandlerFunc {
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
		w.WriteHeader(http.StatusNoContent)
	}
}
