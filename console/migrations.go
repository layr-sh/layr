package console

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// Migrations contains the database migrations for console.
var Migrations = []core.DatabaseMigration{
	{
		Service:     "console",
		Version:     1,
		Description: "Initialize console schema, dynamic config, users, sessions, and passkeys",
		UpSQL: `
CREATE SCHEMA IF NOT EXISTS console;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS console.config (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Console Operator Users
CREATE TABLE IF NOT EXISTS console.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    encrypted_mfa_secret BYTEA,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_signed_in_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON console.users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_enabled ON console.users(is_enabled);

-- 3. Console Sessions
CREATE TABLE IF NOT EXISTS console.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES console.users(id) ON DELETE CASCADE,
    session_token_hash TEXT NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

ALTER TABLE console.sessions ADD COLUMN mfa_verified BOOLEAN NOT NULL DEFAULT false;

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
    friendly_name TEXT NOT NULL DEFAULT 'Security Key',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_passkeys_user_id ON console.passkeys(user_id);

-- Acquire the shared transaction lock before row locks, including cascading deletes.
CREATE FUNCTION console.lock_root_user_changes() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(731941821);
    RETURN NULL;
END;
$$;
CREATE TRIGGER lock_root_user_changes BEFORE UPDATE OF is_enabled OR DELETE ON console.users
FOR EACH STATEMENT EXECUTE FUNCTION console.lock_root_user_changes();
CREATE TRIGGER lock_root_user_changes BEFORE UPDATE OF scopes, is_enabled, expires_at, console_user_id OR DELETE ON core.service_accounts
FOR EACH STATEMENT EXECUTE FUNCTION console.lock_root_user_changes();

CREATE FUNCTION console.protect_root_user() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME = 'users' THEN
        IF OLD.is_enabled AND (TG_OP = 'DELETE' OR NOT NEW.is_enabled)
           AND EXISTS (SELECT 1 FROM core.service_accounts WHERE console_user_id = OLD.id
                       AND is_enabled AND scopes @> '["*"]'::jsonb
                       AND (expires_at IS NULL OR expires_at > clock_timestamp()))
           AND NOT EXISTS (
               SELECT 1 FROM console.users u JOIN core.service_accounts sa ON sa.console_user_id = u.id
               WHERE u.id <> OLD.id AND u.is_enabled AND sa.is_enabled
                 AND sa.scopes @> '["*"]'::jsonb
                 AND (sa.expires_at IS NULL OR sa.expires_at > clock_timestamp())
           ) THEN
            RAISE EXCEPTION 'Cannot remove the last enabled root Console User'
                USING ERRCODE = '23514', CONSTRAINT = 'console_root_user_required';
        END IF;
    ELSE
        IF OLD.console_user_id IS NOT NULL AND OLD.is_enabled AND OLD.scopes @> '["*"]'::jsonb
           AND (OLD.expires_at IS NULL OR OLD.expires_at > clock_timestamp())
           AND EXISTS (SELECT 1 FROM console.users WHERE id = OLD.console_user_id AND is_enabled)
           AND (TG_OP = 'DELETE' OR NOT NEW.is_enabled OR NOT NEW.scopes @> '["*"]'::jsonb
                OR NEW.console_user_id IS DISTINCT FROM OLD.console_user_id
                OR NEW.expires_at <= clock_timestamp())
           AND NOT EXISTS (
               SELECT 1 FROM console.users u JOIN core.service_accounts sa ON sa.console_user_id = u.id
               WHERE sa.id <> OLD.id AND u.is_enabled AND sa.is_enabled
                 AND sa.scopes @> '["*"]'::jsonb
                 AND (sa.expires_at IS NULL OR sa.expires_at > clock_timestamp())
           ) THEN
            RAISE EXCEPTION 'Cannot revoke the last root Console User service account'
                USING ERRCODE = '23514', CONSTRAINT = 'console_root_user_required';
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER protect_root_user BEFORE UPDATE OF is_enabled OR DELETE ON console.users
FOR EACH ROW EXECUTE FUNCTION console.protect_root_user();
CREATE TRIGGER protect_root_user BEFORE UPDATE OF scopes, is_enabled, expires_at, console_user_id OR DELETE ON core.service_accounts
FOR EACH ROW EXECUTE FUNCTION console.protect_root_user();
		`,
		DownSQL: `
DROP SCHEMA IF EXISTS console CASCADE;
		`,
	},
}
