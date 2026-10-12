package auth

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// AuthDatabaseMigration defines the database schema for auth.
var AuthDatabaseMigration = core.DatabaseMigration{
	Service:     "auth",
	Version:     1,
	Description: "Initialize auth schema, users, identities, sessions, passkeys, and otps",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS auth;

-- 1. Dynamic Runtime Configuration (Managed via Console)
CREATE TABLE IF NOT EXISTS auth.config (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Core User Identity Table
CREATE TABLE IF NOT EXISTS auth.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email TEXT UNIQUE,
    phone TEXT UNIQUE,
    password_hash TEXT,
    encrypted_mfa_secret TEXT,
    mfa_enabled BOOLEAN NOT NULL DEFAULT false,
    role TEXT NOT NULL DEFAULT 'authenticated',
    is_anonymous BOOLEAN NOT NULL DEFAULT false,
    email_verified_at TIMESTAMPTZ,
    phone_verified_at TIMESTAMPTZ,
    locked_until TIMESTAMPTZ,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON auth.users(email);
CREATE INDEX IF NOT EXISTS idx_users_phone ON auth.users(phone);

-- 3. Federated OAuth Identities (Google, GitHub, Apple, etc.)
CREATE TABLE IF NOT EXISTS auth.identities (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_sign_in_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_identities_user_id ON auth.identities(user_id);

-- 4. Active Refresh Sessions
CREATE TABLE IF NOT EXISTS auth.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    client_id TEXT,
    refresh_token_hash TEXT NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON auth.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_hash ON auth.sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_client_id ON auth.sessions(client_id);

-- 5. WebAuthn Passkey Credentials
CREATE TABLE IF NOT EXISTS auth.passkeys (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    credential_id BYTEA UNIQUE NOT NULL,
    public_key BYTEA NOT NULL,
    counter BIGINT NOT NULL DEFAULT 0,
    transports TEXT[] DEFAULT ARRAY[]::TEXT[],
    friendly_name TEXT NOT NULL DEFAULT 'Passkey',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_passkeys_user ON auth.passkeys(user_id);

-- 6. OTP & Verification Tokens
CREATE TABLE IF NOT EXISTS auth.otps (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    recipient TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    purpose TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_otps_recipient ON auth.otps(recipient, purpose);
`,
	DownSQL: `
DROP SCHEMA IF EXISTS auth CASCADE;
`,
}

// Migrations contains the database migrations for auth.
var Migrations = []core.DatabaseMigration{
	AuthDatabaseMigration,
}
