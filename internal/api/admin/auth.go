package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"astreoGateway/internal/store"
)

const (
	cookieName = "aigw_session"
	cookieTTL  = 24 * time.Hour
)

type contextKey string

const adminIDKey contextKey = "admin_id"

func EnsureSecret(db *sql.DB) (string, error) {
	secret, err := store.GetSetting(db, "cookie_hmac_secret")
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}
	if secret != "" {
		return secret, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	secret = fmt.Sprintf("%x", b)
	if err := store.SetSetting(db, "cookie_hmac_secret", secret); err != nil {
		return "", fmt.Errorf("save secret: %w", err)
	}
	return secret, nil
}

func signCookie(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	sig := fmt.Sprintf("%x", mac.Sum(nil))
	return value + "." + sig
}

func verifyCookie(secret, token string) (string, bool) {
	dot := strings.LastIndex(token, ".")
	if dot < 0 {
		return "", false
	}
	value := token[:dot]
	sig := token[dot+1:]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	expected := fmt.Sprintf("%x", mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	return value, true
}

func makeCookieValue(secret, adminID string) string {
	exp := time.Now().Add(cookieTTL).Unix()
	payload := fmt.Sprintf("%s|%d", adminID, exp)
	enc := base64.StdEncoding.EncodeToString([]byte(payload))
	return signCookie(secret, enc)
}

func parseCookieValue(secret, cookieVal string) (string, error) {
	enc, ok := verifyCookie(secret, cookieVal)
	if !ok {
		return "", fmt.Errorf("invalid signature")
	}
	payload, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("invalid payload")
	}
	parts := strings.SplitN(string(payload), "|", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed payload")
	}
	var exp int64
	if _, err := fmt.Sscanf(parts[1], "%d", &exp); err != nil {
		return "", fmt.Errorf("malformed expiry")
	}
	if time.Now().Unix() > exp {
		return "", fmt.Errorf("expired")
	}
	return parts[0], nil
}

func bootstrapHandler(db *sql.DB, secret string, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			count, err := store.CountAdminUsers(db)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"needed": count == 0})
		case http.MethodPost:
			count, err := store.CountAdminUsers(db)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if count > 0 {
				http.Error(w, `{"error":"admin already exists"}`, http.StatusConflict)
				return
			}
			var req struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
				http.Error(w, `{"error":"username and password required"}`, http.StatusBadRequest)
				return
			}
			if len(req.Password) < 8 {
				http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
				return
			}
			u, err := store.CreateAdminUser(db, req.Username, req.Password)
			if err != nil {
				slog.Error("bootstrap create admin failed", "err", err)
				http.Error(w, `{"error":"failed to create admin"}`, http.StatusInternalServerError)
				return
			}
			setSessionCookie(w, secret, u.ID, cookieSecure)
			writeJSON(w, map[string]any{"id": u.ID, "username": u.Username})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func loginHandler(db *sql.DB, secret string, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
			http.Error(w, `{"error":"username and password required"}`, http.StatusBadRequest)
			return
		}
		u, err := store.GetAdminUserByUsername(db, req.Username)
		if err != nil || u == nil {
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
			return
		}
		if !store.CheckPassword(u.PasswordHash, req.Password) {
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
			return
		}
		setSessionCookie(w, secret, u.ID, cookieSecure)
		writeJSON(w, map[string]any{"id": u.ID, "username": u.Username})
	}
}

func logoutHandler(cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cookieSecure,
			MaxAge:   -1,
			SameSite: http.SameSiteLaxMode,
		})
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func sessionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := r.Context().Value(adminIDKey).(string)
		u, err := store.GetAdminUserByID(db, adminID)
		if err != nil || u == nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"id": u.ID, "username": u.Username})
	}
}

func setSessionCookie(w http.ResponseWriter, secret, adminID string, cookieSecure bool) {
	val := makeCookieValue(secret, adminID)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure,
		MaxAge:   int(cookieTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

func RequireAdmin(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || c.Value == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		adminID, err := parseCookieValue(secret, c.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := withAdminID(r.Context(), adminID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
