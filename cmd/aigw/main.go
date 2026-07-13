package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astreoGateway/internal/api/admin"
	"astreoGateway/internal/api/public"
	"astreoGateway/internal/config"
	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/metrics"
	"astreoGateway/internal/proxy"
	"astreoGateway/internal/routing"
	"astreoGateway/internal/store"
	"astreoGateway/internal/web"

	"github.com/go-chi/chi/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		logger.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	if n, err := store.CountAdminUsers(db); err != nil {
		logger.Warn("could not count admin users", "err", err)
	} else if n == 0 {
		logger.Warn("no admin users — bootstrap at /admin/ or POST /admin/api/bootstrap")
	}

	secret, err := admin.EnsureSecret(db)
	if err != nil {
		logger.Error("failed to ensure admin secret", "err", err)
		os.Exit(1)
	}

	pool := keypool.New()
	if err := pool.Load(db); err != nil {
		logger.Error("failed to load keypool", "err", err)
		os.Exit(1)
	}

	discCtx, discCancel := context.WithCancel(context.Background())
	defer discCancel()
	cache := discovery.New(db, pool, cfg.DiscoveryTTL, cfg.DiscoveryTimeout, logger)
	cache.Start(discCtx)
	defer cache.Stop()

	adminHandler, err := admin.NewRouter(db, secret, cache, pool, cfg.CookieSecure)
	if err != nil {
		logger.Error("failed to create admin router", "err", err)
		os.Exit(1)
	}

	sel := routing.NewSelector(db, cache, pool)
	prox := proxy.New(pool, cfg.ProxyTimeout, cfg.KeyCooldown, logger)
	publicHandler := public.NewRouter(db, cache, prox, sel, logger)

	startedAt := time.Now()
	r := chi.NewRouter()
	r.Get("/healthz", metrics.Healthz(db, startedAt))
	r.Route("/admin/api", func(r chi.Router) {
		r.Mount("/", adminHandler)
	})
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	r.Handle("/admin/*", http.StripPrefix("/admin", web.Handler()))
	r.Route("/v1", func(r chi.Router) {
		r.Mount("/", publicHandler)
	})

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("astreoGateway starting", "addr", cfg.Addr, "db", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received, draining...", "timeout", "10s")

	discCancel()
	cache.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	logger.Info("bye")
}
