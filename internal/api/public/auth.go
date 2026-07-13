package public

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"astreoGateway/internal/store"
)

type contextKey string

const gatewayKeyIDKey contextKey = "gateway_key_id"

func RequireGatewayKey(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" {
				http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
				return
			}
			key, err := store.VerifyGatewayKey(db, token)
			if err != nil || key == nil {
				http.Error(w, `{"error":"invalid gateway key"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), gatewayKeyIDKey, key.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GatewayKeyIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(gatewayKeyIDKey).(string)
	return v
}
