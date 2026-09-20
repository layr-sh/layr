package filestorage

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// FileStorageDatabaseMigrationVersion is the schema version for the file storage subsystem.
const FileStorageDatabaseMigrationVersion = 300

// FileStorageDatabaseMigration defines the database schema for the file_storage subsystem.
var FileStorageDatabaseMigration = core.DatabaseMigration{
	Version:     FileStorageDatabaseMigrationVersion,
	Description: "Initialize file_storage schema, buckets, objects, chunks, s3 credentials, and multipart uploads",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS file_storage;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS file_storage.config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. File Storage Buckets
CREATE TABLE IF NOT EXISTS file_storage.buckets (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name VARCHAR(128) UNIQUE NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT false,
    backend VARCHAR(32) NOT NULL DEFAULT 'database',
    backend_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    allowed_mime_types TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    max_file_size_bytes BIGINT NOT NULL DEFAULT 52428800,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_file_storage_buckets_name ON file_storage.buckets(name);

-- 3. Object Metadata Registry
CREATE TABLE IF NOT EXISTS file_storage.objects (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    bucket_id UUID NOT NULL REFERENCES file_storage.buckets(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum_sha256 VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(bucket_id, object_key)
);

CREATE INDEX IF NOT EXISTS idx_file_storage_objects_lookup ON file_storage.objects(bucket_id, object_key);

-- 4. Database Binary Chunks (512KB Slices for database backend)
CREATE TABLE IF NOT EXISTS file_storage.chunks (
    object_id UUID NOT NULL REFERENCES file_storage.objects(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    chunk_data BYTEA NOT NULL,
    PRIMARY KEY (object_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_file_storage_chunks_object ON file_storage.chunks(object_id, chunk_index);

-- 5. Dedicated S3 Credentials (Linked to core.service_accounts)
CREATE TABLE IF NOT EXISTS file_storage.s3_credentials (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    service_account_id UUID NOT NULL REFERENCES core.service_accounts(id) ON DELETE CASCADE,
    access_key_id VARCHAR(64) UNIQUE NOT NULL,
    encrypted_secret_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_file_storage_s3_credentials_access_key ON file_storage.s3_credentials(access_key_id);
CREATE INDEX IF NOT EXISTS idx_file_storage_s3_credentials_service_account ON file_storage.s3_credentials(service_account_id);

-- 6. Multipart Uploads Registry
CREATE TABLE IF NOT EXISTS file_storage.multipart_uploads (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    bucket_id UUID NOT NULL REFERENCES file_storage.buckets(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    content_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_file_storage_multipart_uploads_lookup ON file_storage.multipart_uploads(bucket_id, object_key);

-- 7. Multipart Upload Parts
CREATE TABLE IF NOT EXISTS file_storage.multipart_parts (
    upload_id UUID NOT NULL REFERENCES file_storage.multipart_uploads(id) ON DELETE CASCADE,
    part_number INT NOT NULL,
    etag TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    chunk_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (upload_id, part_number)
);

-- 8. Enable Row-Level Security on objects
ALTER TABLE file_storage.objects ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies 
        WHERE schemaname = 'file_storage' 
          AND tablename = 'objects' 
          AND policyname = 'storage_public_bucket_policy'
    ) THEN
        CREATE POLICY storage_public_bucket_policy ON file_storage.objects
        FOR ALL
        USING (
            EXISTS (
                SELECT 1 FROM file_storage.buckets
                WHERE file_storage.buckets.id = file_storage.objects.bucket_id
                  AND file_storage.buckets.is_public = true
            )
        )
        WITH CHECK (
            EXISTS (
                SELECT 1 FROM file_storage.buckets
                WHERE file_storage.buckets.id = file_storage.objects.bucket_id
                  AND file_storage.buckets.is_public = true
            )
        );
    END IF;
END $$;
`,
	DownSQL: `
DROP SCHEMA IF EXISTS file_storage CASCADE;
`,
}

// Migrations is the list of all migrations for the file storage subsystem.
var Migrations = []core.DatabaseMigration{
	FileStorageDatabaseMigration,
}
