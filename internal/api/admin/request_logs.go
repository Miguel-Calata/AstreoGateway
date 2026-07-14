package admin

import (
	"net/http"
	"strconv"

	"astreoGateway/internal/metrics"

	"github.com/go-chi/chi/v5"
)

func requestLogsRouter(logs *metrics.LogStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listRequestLogs(logs))
	r.Get("/stats", statsRequestLogs(logs))
	r.Delete("/", clearRequestLogs(logs))
	return r
}

func listRequestLogs(logs *metrics.LogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logs == nil {
			writeJSON(w, metrics.ListResult{Items: []metrics.RequestLog{}})
			return
		}
		q := metrics.ListQuery{
			GatewayKeyID: r.URL.Query().Get("gateway_key_id"),
			ProviderSlug: r.URL.Query().Get("provider_slug"),
			StatusClass:  r.URL.Query().Get("status_class"),
			Directive:    r.URL.Query().Get("directive"),
			Order:        r.URL.Query().Get("order"),
		}
		if v := r.URL.Query().Get("from"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				q.From = n
			}
		}
		if v := r.URL.Query().Get("to"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				q.To = n
			}
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				q.Limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				q.Offset = n
			}
		}
		writeJSON(w, logs.List(q))
	}
}

func statsRequestLogs(logs *metrics.LogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logs == nil {
			writeJSON(w, metrics.StatsResult{})
			return
		}
		q := metrics.StatsQuery{
			Window:  r.URL.Query().Get("window"),
			GroupBy: r.URL.Query().Get("group_by"),
		}
		writeJSON(w, logs.Stats(q))
	}
}

func clearRequestLogs(logs *metrics.LogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logs != nil {
			logs.Clear()
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
