package admin

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func gatewayKeysRouter(db *sql.DB) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listGatewayKeys(db))
	r.Post("/", createGatewayKey(db))
	r.Delete("/{keyID}", deleteGatewayKey(db))
	return r
}

func listGatewayKeys(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys, err := store.ListGatewayKeys(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, keys)
	}
}

func createGatewayKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Label = ""
		}
		token, err := generateToken()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": "failed to generate token"})
			return
		}
		k, err := store.CreateGatewayKey(db, token, req.Label)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]any{
			"id":      k.ID,
			"label":   k.Label,
			"prefix":  k.Prefix,
			"enabled": k.Enabled,
			"token":   token,
		})
	}
}

func deleteGatewayKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "keyID")
		if err := store.DeleteGatewayKey(db, id); err != nil {
			if err.Error() == "gateway_key not found" {
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

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "aigw_" + hex.EncodeToString(b), nil
}
