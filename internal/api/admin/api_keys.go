package admin

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func apiKeysRouter(db *sql.DB, pool *keypool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Route("/{keyID}", func(r chi.Router) {
		r.Get("/", getAPIKey(db))
		r.Put("/", updateAPIKey(db, pool))
		r.Delete("/", deleteAPIKey(db, pool))
	})
	return r
}

func providerAPIKeysRouter(db *sql.DB, pool *keypool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listAPIKeys(db))
	r.Post("/", createAPIKey(db, pool))
	return r
}

func reloadPool(pool *keypool.Pool, db *sql.DB) {
	if pool == nil {
		return
	}
	if err := pool.Load(db); err != nil {
		slog.Error("keypool: reload after api_key change", "err", err)
	}
}

func listAPIKeys(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID := chi.URLParam(r, "providerID")
		keys, err := store.ListAPIKeysByProvider(db, providerID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, keys)
	}
}

func createAPIKey(db *sql.DB, pool *keypool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID := chi.URLParam(r, "providerID")
		var k model.APIKey
		if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		k.ProviderID = providerID
		if k.Value == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "key_value is required"})
			return
		}
		if err := store.CreateAPIKey(db, &k); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		reloadPool(pool, db)
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, k)
	}
}

func getAPIKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "keyID")
		k, err := store.GetAPIKeyByID(db, id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		if k == nil {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"error": "api_key not found"})
			return
		}
		writeJSON(w, k)
	}
}

func updateAPIKey(db *sql.DB, pool *keypool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "keyID")
		existing, err := store.GetAPIKeyByID(db, id)
		if err != nil || existing == nil {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"error": "api_key not found"})
			return
		}
		var k model.APIKey
		if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		k.ID = id
		k.ProviderID = existing.ProviderID
		if err := store.UpdateAPIKey(db, &k); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		reloadPool(pool, db)
		writeJSON(w, k)
	}
}

func deleteAPIKey(db *sql.DB, pool *keypool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "keyID")
		if err := store.DeleteAPIKey(db, id); err != nil {
			if err.Error() == "api_key not found" {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, map[string]any{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		reloadPool(pool, db)
		w.WriteHeader(http.StatusNoContent)
	}
}
