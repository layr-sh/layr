package image

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// ImageDatabaseMigrationVersion is the schema version for the image subsystem.
const ImageDatabaseMigrationVersion = 700

// ImageDatabaseMigration defines the database schema for the image subsystem.
var ImageDatabaseMigration = core.DatabaseMigration{
	Version:     ImageDatabaseMigrationVersion,
	Description: "Initialize image schema, dynamic config, presets, and cache entries",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS image;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS image.config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Named Processing Presets
CREATE TABLE IF NOT EXISTS image.presets (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name VARCHAR(128) UNIQUE NOT NULL,
    processing_options TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 3. Transformation Cache Index
CREATE TABLE IF NOT EXISTS image.cache_entries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    cache_key VARCHAR(255) UNIQUE NOT NULL,
    source_url TEXT NOT NULL,
    options_hash VARCHAR(64) NOT NULL,
    content_type VARCHAR(64) NOT NULL,
    byte_size BIGINT NOT NULL,
    etag VARCHAR(128) NOT NULL,
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

// Migrations is the list of all image migrations.
var Migrations = []core.DatabaseMigration{
	ImageDatabaseMigration,
}
