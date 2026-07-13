-- 001_init.sql: initial schema for providers, keys, aliases, gateway keys, admin, settings.

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS providers (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE,
	protocol      TEXT NOT NULL CHECK (protocol IN ('openai', 'anthropic')),
	base_url      TEXT NOT NULL,
	enabled       INTEGER NOT NULL DEFAULT 1,
	headers       TEXT NOT NULL DEFAULT '{}',  -- JSON object of extra headers
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
	id            TEXT PRIMARY KEY,
	provider_id   TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
	label         TEXT NOT NULL DEFAULT '',
	key_value     TEXT NOT NULL,
	priority      INTEGER NOT NULL DEFAULT 0,
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_keys_provider ON api_keys(provider_id);

CREATE TABLE IF NOT EXISTS aliases (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE,
	routing       TEXT NOT NULL DEFAULT 'failover' CHECK (routing IN ('random', 'round_robin', 'priority', 'failover')),
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alias_targets (
	alias_id      TEXT NOT NULL REFERENCES aliases(id) ON DELETE CASCADE,
	provider_id   TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
	model_name    TEXT NOT NULL,
	position      INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (alias_id, provider_id, model_name)
);
CREATE INDEX IF NOT EXISTS idx_alias_targets_alias ON alias_targets(alias_id);

CREATE TABLE IF NOT EXISTS gateway_keys (
	id            TEXT PRIMARY KEY,
	label         TEXT NOT NULL DEFAULT '',
	key_hash      TEXT NOT NULL,  -- sha256 hex of the bearer token
	prefix        TEXT NOT NULL DEFAULT '',  -- first 8 chars for display lookup
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admin_users (
	id            TEXT PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,  -- bcrypt hash
	created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
	key           TEXT PRIMARY KEY,
	value         TEXT NOT NULL,
	updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);