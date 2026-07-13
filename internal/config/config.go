package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config holds runtime configuration. Most fields come from env vars or flags;
// long-lived settings (providers, aliases, etc.) live in SQLite.
type Config struct {
	Addr             string
	DBPath           string
	LogLevel         slog.Level
	DiscoveryTTL     time.Duration
	DiscoveryTimeout time.Duration
	ProxyTimeout     time.Duration
	KeyCooldown      time.Duration
	CookieSecure     bool
}

// Load parses env vars and flags into a Config.
func Load() (*Config, error) {
	fs := flag.NewFlagSet("aigw", flag.ContinueOnError)
	addr := fs.String("addr", envOr("AIGW_ADDR", ":18473"), "HTTP listen address")
	dbPath := fs.String("db", envOr("AIGW_DB", "data/aigw.db"), "SQLite database path")
	logLevel := fs.String("log-level", envOr("AIGW_LOG_LEVEL", "info"), "log level: debug|info|warn|error")
	discoveryTTL := fs.String("discovery-ttl", envOr("AIGW_DISCOVERY_TTL", "5m"), "model cache refresh interval")
	discoveryTimeout := fs.String("discovery-timeout", envOr("AIGW_DISCOVERY_TIMEOUT", "10s"), "HTTP timeout for upstream model fetches")
	proxyTimeout := fs.String("proxy-timeout", envOr("AIGW_PROXY_TIMEOUT", "120s"), "HTTP timeout for proxied requests")
	keyCooldown := fs.String("key-cooldown", envOr("AIGW_KEY_COOLDOWN", "30s"), "cooldown duration after 429/5xx errors")
	cookieSecure := fs.Bool("cookie-secure", envOrBool("AIGW_COOKIE_SECURE", false), "set Secure flag on admin session cookie (use behind HTTPS)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	ttl, err := time.ParseDuration(*discoveryTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid discovery-ttl %q: %w", *discoveryTTL, err)
	}
	timeout, err := time.ParseDuration(*discoveryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid discovery-timeout %q: %w", *discoveryTimeout, err)
	}
	proxy, err := time.ParseDuration(*proxyTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy-timeout %q: %w", *proxyTimeout, err)
	}
	cooldown, err := time.ParseDuration(*keyCooldown)
	if err != nil {
		return nil, fmt.Errorf("invalid key-cooldown %q: %w", *keyCooldown, err)
	}

	cfg := &Config{
		Addr:             *addr,
		DBPath:           *dbPath,
		DiscoveryTTL:     ttl,
		DiscoveryTimeout: timeout,
		ProxyTimeout:     proxy,
		KeyCooldown:      cooldown,
		CookieSecure:     *cookieSecure,
	}
	switch strings.ToLower(*logLevel) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn", "warning":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}