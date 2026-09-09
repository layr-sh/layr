package auth

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// AuthDatabaseMigrationVersion is the schema version for the layr_auth subsystem.
const AuthDatabaseMigrationVersion = 200

// AuthDatabaseMigration defines the database schema for layr_auth.
var AuthDatabaseMigration = core.DatabaseMigration{
	Version:     AuthDatabaseMigrationVersion,
	Description: "Initialize layr_auth schema, users, identities, sessions, passkeys, and otps",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS layr_auth;

-- 1. Dynamic Runtime Configuration (Managed via Console)
CREATE TABLE IF NOT EXISTS layr_auth.config (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    key VARCHAR(128) NOT NULL UNIQUE,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Core User Identity Table
CREATE TABLE IF NOT EXISTS layr_auth.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(32) UNIQUE,
    password_hash VARCHAR(255),
    role VARCHAR(64) NOT NULL DEFAULT 'authenticated',
    is_anonymous BOOLEAN NOT NULL DEFAULT false,
    email_verified_at TIMESTAMPTZ,
    phone_verified_at TIMESTAMPTZ,
    locked_until TIMESTAMPTZ,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_users_email ON layr_auth.users(email);
CREATE INDEX IF NOT EXISTS idx_layr_users_phone ON layr_auth.users(phone);

-- 3. Federated OAuth Identities (Google, GitHub, Apple, etc.)
CREATE TABLE IF NOT EXISTS layr_auth.identities (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES layr_auth.users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_sign_in_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_layr_identities_user_id ON layr_auth.identities(user_id);

-- 4. Active Refresh Sessions
CREATE TABLE IF NOT EXISTS layr_auth.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES layr_auth.users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_sessions_user ON layr_auth.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_layr_sessions_hash ON layr_auth.sessions(refresh_token_hash);

-- 5. WebAuthn Passkey Credentials
CREATE TABLE IF NOT EXISTS layr_auth.passkeys (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES layr_auth.users(id) ON DELETE CASCADE,
    credential_id BYTEA UNIQUE NOT NULL,
    public_key BYTEA NOT NULL,
    counter BIGINT NOT NULL DEFAULT 0,
    transports TEXT[] DEFAULT ARRAY[]::TEXT[],
    friendly_name VARCHAR(128) NOT NULL DEFAULT 'Passkey',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_passkeys_user ON layr_auth.passkeys(user_id);

-- 6. OTP & Verification Tokens
CREATE TABLE IF NOT EXISTS layr_auth.otps (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    recipient VARCHAR(255) NOT NULL,
    code_hash VARCHAR(255) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_otps_recipient ON layr_auth.otps(recipient, purpose);
`,
	DownSQL: `
DROP TABLE IF EXISTS layr_auth.otps CASCADE;
DROP TABLE IF EXISTS layr_auth.passkeys CASCADE;
DROP TABLE IF EXISTS layr_auth.sessions CASCADE;
DROP TABLE IF EXISTS layr_auth.identities CASCADE;
DROP TABLE IF EXISTS layr_auth.users CASCADE;
DROP TABLE IF EXISTS layr_auth.config CASCADE;
DROP SCHEMA IF EXISTS layr_auth CASCADE;
`,
}

// Migrations contains the DDL migrations for layr_auth.
var Migrations = []core.DatabaseMigration{
	AuthDatabaseMigration,
}
