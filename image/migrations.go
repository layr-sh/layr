package image

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// ImageDatabaseMigration defines the database schema for the image subsystem.
var ImageDatabaseMigration = core.DatabaseMigration{
	Service:     "image",
	Version:     1,
	Description: "Initialize image schema, dynamic config, presets, and cache entries",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS image;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS image.config (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Named Processing Presets
CREATE TABLE IF NOT EXISTS image.presets (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT UNIQUE NOT NULL,
    processing_options TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 3. Transformation Cache Index
CREATE TABLE IF NOT EXISTS image.cache_entries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    cache_key TEXT UNIQUE NOT NULL,
    source_url TEXT NOT NULL,
    options_hash TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL,
    etag TEXT NOT NULL,
    last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_image_cache_lookup ON image.cache_entries (cache_key);
CREATE INDEX IF NOT EXISTS idx_image_cache_lru ON image.cache_entries (last_accessed_at);
`,
	DownSQL: `
DROP SCHEMA IF EXISTS image CASCADE;
`,
}

// Migrations contains the database migrations for image.
var Migrations = []core.DatabaseMigration{
	ImageDatabaseMigration,
}
