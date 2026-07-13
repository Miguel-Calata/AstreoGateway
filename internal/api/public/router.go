package public

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

func NewRouter(db *sql.DB, cache *discovery.Cache, prox *proxy.Proxy, sel *routing.Selector, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(RequireGatewayKey(db))
	r.Get("/models", listModels(db, cache))
	r.Post("/chat/completions", chatHandler(prox, sel, logger))
	r.HandleFunc("/embeddings", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"embeddings not yet available (milestone 7)"}`, http.StatusNotImplemented)
	})
	return r
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
