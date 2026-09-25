package function

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// FunctionDatabaseMigration defines the database schema for the function subsystem.
var FunctionDatabaseMigration = core.DatabaseMigration{
	Service:     "function",
	Version:     1,
	Description: "Initialize function schema, dynamic config, endpoints, and deployments",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS function;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS function.config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Endpoints Entity Table
CREATE TABLE IF NOT EXISTS function.endpoints (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name VARCHAR(128) UNIQUE NOT NULL,
    description TEXT,
    runtime VARCHAR(64) NOT NULL DEFAULT 'workerd',
    entrypoint VARCHAR(128) NOT NULL DEFAULT 'index.js',
    memory_limit_mb INT NOT NULL DEFAULT 128,
    timeout_seconds INT NOT NULL DEFAULT 30,
    is_public BOOLEAN NOT NULL DEFAULT true,
    active_deployment_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 3. Deployments & Code Bundles (Immutable version history)
CREATE TABLE IF NOT EXISTS function.deployments (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    endpoint_id UUID NOT NULL REFERENCES function.endpoints(id) ON DELETE CASCADE,
    version INT NOT NULL,
    bundle_format VARCHAR(32) NOT NULL DEFAULT 'raw',
    bundle_content BYTEA NOT NULL,
    bundle_hash VARCHAR(64) NOT NULL,
    bundle_files JSONB NOT NULL DEFAULT '{}'::jsonb,
    environment_variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    workerd_runtime_config JSONB,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_endpoint_version UNIQUE (endpoint_id, version)
);

-- Foreign key constraint for active_deployment_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_endpoints_active_deployment'
    ) THEN
        ALTER TABLE function.endpoints
            ADD CONSTRAINT fk_endpoints_active_deployment
            FOREIGN KEY (active_deployment_id)
            REFERENCES function.deployments(id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_function_deployments_endpoint ON function.deployments(endpoint_id, version DESC);

-- 4. Invocations & Execution Logs
CREATE TABLE IF NOT EXISTS function.executions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    endpoint_id UUID NOT NULL REFERENCES function.endpoints(id) ON DELETE CASCADE,
    deployment_id UUID REFERENCES function.deployments(id) ON DELETE SET NULL,
    method VARCHAR(16) NOT NULL,
    path TEXT NOT NULL,
    status_code INT NOT NULL,
    duration_ms INT NOT NULL,
    stdout TEXT NOT NULL DEFAULT '',
    stderr TEXT NOT NULL DEFAULT '',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_function_executions_endpoint ON function.executions(endpoint_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_function_executions_created_at ON function.executions(created_at DESC);

-- 5. Standalone Custom Domains
CREATE TABLE IF NOT EXISTS function.custom_domains (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    domain VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_function_custom_domains_domain ON function.custom_domains(domain);

-- 6. Custom Domain Route Mappings
CREATE TABLE IF NOT EXISTS function.custom_domain_routes (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    custom_domain_id UUID NOT NULL REFERENCES function.custom_domains(id) ON DELETE CASCADE,
    endpoint_id UUID NOT NULL REFERENCES function.endpoints(id) ON DELETE CASCADE,
    path_prefix VARCHAR(255) NOT NULL DEFAULT '/',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_function_custom_domain_routes_prefix UNIQUE(custom_domain_id, path_prefix)
);

CREATE INDEX IF NOT EXISTS idx_function_custom_domain_routes_domain ON function.custom_domain_routes(custom_domain_id);
CREATE INDEX IF NOT EXISTS idx_function_custom_domain_routes_endpoint ON function.custom_domain_routes(endpoint_id);
`,
	DownSQL: `
DROP SCHEMA IF EXISTS function CASCADE;
`,
}

// Migrations is the list of all function migrations.
var Migrations = []core.DatabaseMigration{
	FunctionDatabaseMigration,
}
