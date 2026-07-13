package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"astreoGateway/internal/model"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func aliasesRouter(db *sql.DB) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listAliases(db))
	r.Post("/", createAlias(db))
	r.Route("/{aliasID}", func(r chi.Router) {
		r.Get("/", getAlias(db))
		r.Put("/", updateAlias(db))
		r.Delete("/", deleteAlias(db))
	})
	return r
}

func listAliases(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aliases, err := store.ListAliases(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, aliases)
	}
}

func createAlias(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var a model.Alias
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		if a.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "name is required"})
			return
		}
		if a.Routing == "" {
			a.Routing = "failover"
		}
		if a.Targets == nil {
			a.Targets = []model.AliasTarget{}
		}
		if err := store.CreateAlias(db, &a); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, a)
	}
}

func getAlias(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "aliasID")
		a, err := store.GetAliasByID(db, id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		if a == nil {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"error": "alias not found"})
			return
		}
		writeJSON(w, a)
	}
}

func updateAlias(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "aliasID")
		var a model.Alias
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "invalid JSON"})
			return
		}
		a.ID = id
		if a.Targets == nil {
			a.Targets = []model.AliasTarget{}
		}
		if err := store.UpdateAlias(db, &a); err != nil {
			if err.Error() == "alias not found" {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, map[string]any{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, a)
	}
}

func deleteAlias(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "aliasID")
		if err := store.DeleteAlias(db, id); err != nil {
			if err.Error() == "alias not found" {
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
