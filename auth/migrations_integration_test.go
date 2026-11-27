package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestAuthMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	tablesToCheck := []string{"config", "users", "identities", "sessions", "passkeys", "otps"}

	// 1. Verify that auth tables exist
	t.Run("VerifyAuthTablesExist", func(t *testing.T) {
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables
					WHERE table_schema = 'auth' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected auth.%s to exist", tableName)
		}
	})

	// 2. Test rollback of auth migrations
	t.Run("RollbackAuthMigrations", func(t *testing.T) {
		for idx := len(Migrations) - 1; idx >= 0; idx-- {
			databaseMigration := Migrations[idx]
			_, downErr := db.Exec(ctx, databaseMigration.DownSQL)
			require.NoError(t, downErr)
		}

		// Verify schema is dropped
		var schemaExists bool
		err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.schemata
				WHERE schema_name = 'auth'
			);
		`).Scan(&schemaExists)
		require.NoError(t, err)
		require.False(t, schemaExists)
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyAuthMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
			require.NoError(t, upErr)
		}

		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables
					WHERE table_schema = 'auth' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected auth.%s to exist after reapply", tableName)
		}
	})

	t.Run("VerifySessionDefaults", func(t *testing.T) {
		var hasLastUsedAt bool
		var scope string
		queryErr := db.QueryRow(ctx, `
			WITH new_user AS (INSERT INTO auth.users DEFAULT VALUES RETURNING id),
			new_session AS (
				INSERT INTO auth.sessions (user_id, refresh_token_hash, expires_at)
				SELECT id, 'test-refresh-token-hash', clock_timestamp() + interval '1 hour'
				FROM new_user
				RETURNING last_used_at, scope
			)
			SELECT last_used_at IS NOT NULL, scope FROM new_session;
		`).Scan(&hasLastUsedAt, &scope)
		require.NoError(t, queryErr)
		require.True(t, hasLastUsedAt)
		require.Empty(t, scope)
	})
}
