-- Public routing prefix for provider:model IDs (stable, URL-safe).
-- Empty values are backfilled by store.ensureProviderSlugs after migrate.
ALTER TABLE providers ADD COLUMN slug TEXT NOT NULL DEFAULT '';
