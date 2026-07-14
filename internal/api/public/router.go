package public

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/metrics"
	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

func NewRouter(db *sql.DB, cache *discovery.Cache, prox *proxy.Proxy, sel *routing.Selector, logger *slog.Logger, logs *metrics.LogStore) http.Handler {
	r := chi.NewRouter()

	r.Use(RequireGatewayKey(db))
	r.Use(metrics.AccessLog(logs))
	r.Use(attachGatewayKeyToLog)
	r.Get("/models", listModels(db, cache))
	r.Post("/chat/completions", chatHandler(prox, sel, logger))
	r.Post("/embeddings", embeddingsHandler(prox, sel, logger))
	return r
}

func attachGatewayKeyToLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if entry := metrics.EntryFromContext(r.Context()); entry != nil {
			entry.GatewayKeyID = GatewayKeyIDFromContext(r.Context())
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
