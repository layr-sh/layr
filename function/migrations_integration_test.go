package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	tablesToCheck := []string{"config", "endpoints", "deployments"}

	// 1. Verify that function tables exist
	t.Run("VerifyFunctionTablesExist", func(t *testing.T) {
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'function' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected function.%s table to exist", tableName)
		}
	})

	// 2. Test rollback of function migrations
	t.Run("RollbackFunctionMigrations", func(t *testing.T) {
		for index := len(Migrations) - 1; index >= 0; index-- {
			databaseMigration := Migrations[index]
			_, downErr := db.Exec(ctx, databaseMigration.DownSQL)
			require.NoError(t, downErr, "failed rollback of migration %d", databaseMigration.Version)
		}

		// Verify function schema was dropped
		var schemaExists bool
		schemaErr := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.schemata 
				WHERE schema_name = 'function'
			);
		`).Scan(&schemaExists)
		require.NoError(t, schemaErr)
		require.False(t, schemaExists, "expected function schema to be dropped after rollback")
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyFunctionMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
			require.NoError(t, upErr, "failed re-applying migration %d", databaseMigration.Version)
		}

		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'function' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected function.%s table to exist after reapply", tableName)
		}
	})
}
