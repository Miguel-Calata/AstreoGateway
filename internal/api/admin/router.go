package admin

import (
	"database/sql"
	"net/http"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"

	"github.com/go-chi/chi/v5"
)

func NewRouter(db *sql.DB, secret string, cache *discovery.Cache, pool *keypool.Pool) (http.Handler, error) {
	r := chi.NewRouter()

	r.Get("/bootstrap", bootstrapHandler(db))
	r.Post("/bootstrap", bootstrapHandler(db))
	r.Post("/login", loginHandler(db, secret))
	r.Post("/logout", logoutHandler)
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return RequireAdmin(secret, next)
		})
		r.Get("/session", sessionHandler(db))
		r.Mount("/providers", providersRouter(db))
		r.Route("/providers/{providerID}/api-keys", func(r chi.Router) {
			r.Mount("/", providerAPIKeysRouter(db, pool))
		})
		r.Mount("/api-keys", apiKeysRouter(db, pool))
		r.Mount("/aliases", aliasesRouter(db))
		r.Mount("/gateway-keys", gatewayKeysRouter(db))
		r.Mount("/discovery", discoveryRouter(cache))
	})
	return r, nil
}
