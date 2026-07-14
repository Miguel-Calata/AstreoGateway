package metrics

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

type ctxKey int

const entryKey ctxKey = 1

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func EntryFromContext(ctx context.Context) *RequestLog {
	v, _ := ctx.Value(entryKey).(*RequestLog)
	return v
}

func withEntry(ctx context.Context, e *RequestLog) context.Context {
	return context.WithValue(ctx, entryKey, e)
}

func AccessLog(store *LogStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			id := NewRequestID()
			entry := &RequestLog{
				ID:        id,
				RequestID: id,
				Ts:        start.UnixMilli(),
				Method:    r.Method,
				Path:      r.URL.Path,
				ClientIP:  clientIP(r),
			}
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			ctx := withEntry(r.Context(), entry)
			next.ServeHTTP(rec, r.WithContext(ctx))

			// Prefer logical Outcome status when the proxy reported a failure
			// (e.g. mid-stream cut: client saw 200 SSE, log records 502 + error_class).
			if entry.ErrorClass != "" && entry.Status > 0 {
				// keep Outcome status
			} else if entry.Status == 0 || rec.wroteHeader {
				entry.Status = rec.status
			}
			entry.DurationMs = time.Since(start).Milliseconds()
			if entry.Attempts == 0 && (entry.ResolvedProvider != "" || entry.Directive != "") {
				entry.Attempts = 1
			}
			store.Append(*entry)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ApplyProxyOutcome(entry *RequestLog, providerSlug, modelName, aliasName, errorClass string, status int, tokensPrompt, tokensCompletion int, stream bool, attempts []Attempt) {
	if entry == nil {
		return
	}
	if providerSlug != "" {
		entry.ResolvedProvider = providerSlug
	}
	if modelName != "" {
		entry.ResolvedModel = modelName
	}
	if aliasName != "" {
		entry.AliasName = aliasName
	}
	if errorClass != "" {
		entry.ErrorClass = errorClass
	}
	if status > 0 {
		entry.Status = status
	}
	entry.TokensPrompt = tokensPrompt
	entry.TokensCompletion = tokensCompletion
	entry.Stream = stream
	if n := len(attempts); n > 0 {
		entry.Attempts = n
		entry.AttemptsDetail = attempts
	}
}
