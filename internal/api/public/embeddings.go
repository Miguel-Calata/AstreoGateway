package public

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"
)

func embeddingsHandler(prox *proxy.Proxy, sel *routing.Selector, _ *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "failed to read body")
			return
		}
		if len(body) == 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "empty body")
			return
		}

		directive, err := peekModel(body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		prox.Embeddings(r.Context(), w, sel, directive, body)
	}
}

func peekModel(body []byte) (string, error) {
	var peek struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return "", fmt.Errorf("invalid JSON body")
	}
	if peek.Model == "" {
		return "", errMissingModel
	}
	return peek.Model, nil
}
