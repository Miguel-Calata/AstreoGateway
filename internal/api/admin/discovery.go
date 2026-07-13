package admin

import (
	"net/http"
	"strings"

	"astreoGateway/internal/discovery"

	"github.com/go-chi/chi/v5"
)

func discoveryRouter(cache *discovery.Cache) http.Handler {
	r := chi.NewRouter()
	r.Get("/models", discoveryModelsHandler(cache))
	r.Get("/stale", discoveryStaleHandler(cache))
	r.Post("/refresh", discoveryRefreshHandler(cache))
	return r
}

func discoveryModelsHandler(cache *discovery.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := cache.Snapshot()
		writeJSON(w, snapshot)
	}
}

func discoveryStaleHandler(cache *discovery.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stale := cache.StaleTargets()
		writeJSON(w, stale)
	}
}

func discoveryRefreshHandler(cache *discovery.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID := r.URL.Query().Get("provider")
		if providerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"error": "provider query param required"})
			return
		}
		if err := cache.Refresh(r.Context(), providerID); err != nil {
			msg := err.Error()
			status := http.StatusInternalServerError
			if strings.Contains(msg, "provider not found") {
				status = http.StatusNotFound
			} else if strings.Contains(msg, "provider is disabled") {
				status = http.StatusBadRequest
			}
			w.WriteHeader(status)
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		writeJSON(w, map[string]string{"status": "refreshed", "provider": providerID})
	}
}
