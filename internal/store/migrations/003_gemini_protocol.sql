-- 003_gemini_protocol.sql: allow 'gemini' as provider protocol by removing
-- the CHECK constraint entirely (validation is done in the application
-- layer via protocol.Validate). SQLite cannot ALTER a CHECK constraint;
-- a table rebuild is required.
--
-- Note: DROP TABLE with foreign_keys=ON performs an implicit DELETE that
-- cascades to api_keys and alias_targets. We save and restore child data.

CREATE TEMP TABLE _api_keys_backup AS
SELECT id, provider_id, label, key_value, priority, enabled, created_at FROM api_keys;

CREATE TEMP TABLE _alias_targets_backup AS
SELECT alias_id, provider_id, model_name, position FROM alias_targets;

CREATE TABLE providers_new (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    slug       TEXT NOT NULL DEFAULT '',
    protocol   TEXT NOT NULL,
    base_url   TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    headers    TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO providers_new (id, name, slug, protocol, base_url, enabled, headers, created_at, updated_at)
SELECT id, name, slug, protocol, base_url, enabled, headers, created_at, updated_at FROM providers;

DROP TABLE providers;

ALTER TABLE providers_new RENAME TO providers;

INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled, created_at)
SELECT id, provider_id, label, key_value, priority, enabled, created_at FROM _api_keys_backup;

INSERT INTO alias_targets (alias_id, provider_id, model_name, position)
SELECT alias_id, provider_id, model_name, position FROM _alias_targets_backup;

DROP TABLE _api_keys_backup;
DROP TABLE _alias_targets_backup;