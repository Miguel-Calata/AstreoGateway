package public

import (
	"database/sql"
	"net/http"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/store"
)

type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type modelsResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

func listModels(db *sql.DB, cache *discovery.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheModels := cache.Models()

		providers, err := store.ListProviders(db)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		idToName := make(map[string]string, len(providers))
		for _, p := range providers {
			idToName[p.ID] = p.Name
		}

		aliases, err := store.ListAliases(db)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		data := make([]modelEntry, 0, len(cacheModels)+len(aliases))
		for _, m := range cacheModels {
			prefix := idToName[m.ProviderID]
			if prefix == "" {
				prefix = m.ProviderID
			}
			data = append(data, modelEntry{
				ID:      prefix + ":" + m.ModelID,
				Object:  "model",
				OwnedBy: m.OwnedBy,
			})
		}
		for _, a := range aliases {
			if !a.Enabled {
				continue
			}
			data = append(data, modelEntry{
				ID:      a.Name,
				Object:  "model",
				OwnedBy: "alias",
			})
		}

		writeJSON(w, modelsResponse{Object: "list", Data: data})
	}
}
