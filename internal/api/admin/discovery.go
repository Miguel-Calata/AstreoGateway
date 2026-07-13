package admin

import (
	"net/http"

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
			http.Error(w, `{"error":"provider query param required"}`, http.StatusBadRequest)
			return
		}
		if err := cache.Refresh(r.Context(), providerID); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "refreshed", "provider": providerID})
	}
}
