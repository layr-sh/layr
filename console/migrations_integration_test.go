package console

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	tablesToCheck := []string{"config", "users", "sessions", "passkeys"}

	// 1. Verify that console tables exist
	t.Run("VerifyConsoleTablesExist", func(t *testing.T) {
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'console' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected console.%s table to exist", tableName)
		}
	})

	// 2. Test rollback of console migrations
	t.Run("RollbackConsoleMigrations", func(t *testing.T) {
		for index := len(Migrations) - 1; index >= 0; index-- {
			databaseMigration := Migrations[index]
			_, downErr := db.Exec(ctx, databaseMigration.DownSQL)
			require.NoError(t, downErr, "failed rollback of migration %d", databaseMigration.Version)
		}

		// Verify console tables were dropped
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'console' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.False(t, tableExists, "expected console.%s table to be dropped", tableName)
		}
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyConsoleMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
			require.NoError(t, upErr, "failed reapplication of migration %d", databaseMigration.Version)
		}
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'console' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected console.%s table to exist after reapply", tableName)
		}
	})
}
