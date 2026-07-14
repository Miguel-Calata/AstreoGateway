package public

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"astreoGateway/internal/metrics"
	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"
)

const maxBodySize = 10 * 1024 * 1024

func chatHandler(prox *proxy.Proxy, sel *routing.Selector, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entry := metrics.EntryFromContext(r.Context())
		if entry != nil {
			entry.GatewayKeyID = GatewayKeyIDFromContext(r.Context())
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "failed to read body")
			return
		}
		if len(body) == 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "empty body")
			return
		}

		directive, isStream, err := peekModelAndStream(body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if entry != nil {
			entry.Directive = directive
			entry.Stream = isStream
		}

		out := prox.ChatCompletions(r.Context(), w, sel, directive, body, isStream)
		applyOutcome(entry, out)
	}
}

func applyOutcome(entry *metrics.RequestLog, out *proxy.Outcome) {
	if entry == nil || out == nil {
		return
	}
	attempts := make([]metrics.Attempt, len(out.Attempts))
	for i, a := range out.Attempts {
		attempts[i] = metrics.Attempt{
			ProviderSlug: a.ProviderSlug,
			ModelName:    a.ModelName,
			KeyID:        a.KeyID,
			Status:       a.Status,
			FailClass:    a.FailClass,
			DurationMs:   a.DurationMs,
		}
	}
	metrics.ApplyProxyOutcome(
		entry,
		out.ProviderSlug,
		out.ModelName,
		out.AliasName,
		out.ErrorClass,
		out.Status,
		out.TokensPrompt,
		out.TokensCompletion,
		out.Stream,
		attempts,
	)
}

func peekModelAndStream(body []byte) (string, bool, error) {
	var peek struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return "", false, &json.SyntaxError{}
	}
	if peek.Model == "" {
		return "", false, errMissingModel
	}
	return peek.Model, peek.Stream, nil
}

type jsonSyntaxError struct{}

func (e *jsonSyntaxError) Error() string { return "invalid JSON body" }

var errMissingModel = &missingModelError{}

type missingModelError struct{}

func (e *missingModelError) Error() string { return "missing required field: model" }

func writeJSONError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    errType,
		},
	})
}

func isJSONSyntaxError(err error) bool {
	_, ok := err.(*jsonSyntaxError)
	if ok {
		return true
	}
	_, ok = err.(*json.SyntaxError)
	return ok || (err != nil && strings.Contains(err.Error(), "invalid character"))
}
