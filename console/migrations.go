// Package console provides the embedded single-project console user control plane, operational dashboard, and dynamic runtime configuration manager.
package console

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// ConsoleDatabaseMigration defines the database schema for the console subsystem.
var ConsoleDatabaseMigration = core.DatabaseMigration{
	Service:     "console",
	Version:     1,
	Description: "Initialize console schema, dynamic config, users, sessions, and passkeys",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS console;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS console.config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Console Operator Users
CREATE TABLE IF NOT EXISTS console.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    encrypted_mfa_secret BYTEA,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON console.users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_enabled ON console.users(is_enabled);

-- 3. Console Sessions
CREATE TABLE IF NOT EXISTS console.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES console.users(id) ON DELETE CASCADE,
    session_token_hash VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_sessions_hash ON console.sessions(session_token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON console.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON console.sessions(expires_at);

-- 4. Console Passkeys
CREATE TABLE IF NOT EXISTS console.passkeys (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES console.users(id) ON DELETE CASCADE,
    credential_id BYTEA UNIQUE NOT NULL,
    public_key BYTEA NOT NULL,
    counter BIGINT NOT NULL DEFAULT 0,
    transports TEXT[] DEFAULT ARRAY[]::TEXT[],
    friendly_name VARCHAR(128) NOT NULL DEFAULT 'Security Key',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_passkeys_user_id ON console.passkeys(user_id);
`,
	DownSQL: `
DROP SCHEMA IF EXISTS console CASCADE;
`,
}

// Migrations contains all migration definitions for console service.
var Migrations = []core.DatabaseMigration{
	ConsoleDatabaseMigration,
}
