package auth

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestAuthMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	expectedTables := []string{
		"config",
		"users",
		"identities",
		"sessions",
		"passkeys",
		"otps",
	}

	// 1. Verify that auth tables exist
	t.Run("VerifyAuthTablesExist", func(t *testing.T) {
		for _, tableName := range expectedTables {
			var exists bool
			err := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'auth' AND table_name = $1
				)
			`, tableName).Scan(&exists)
			if err != nil {
				t.Fatalf("failed to query table existence for %s: %v", tableName, err)
			}
			if !exists {
				t.Fatalf("table auth.%s does not exist after running migrations", tableName)
			}
		}
	})

	// 2. Test rollback of auth migrations
	t.Run("RollbackAuthMigrations", func(t *testing.T) {
		for index := len(Migrations) - 1; index >= 0; index-- {
			databaseMigration := Migrations[index]
			if _, downErr := db.Exec(ctx, databaseMigration.DownSQL); downErr != nil {
				t.Fatalf("failed rollback of migration %d: %v", databaseMigration.Version, downErr)
			}
		}

		var schemaExists bool
		schemaErr := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.schemata 
				WHERE schema_name = 'auth'
			);
		`).Scan(&schemaExists)
		if schemaErr != nil || schemaExists {
			t.Fatalf("expected auth schema to be dropped after rollback, got exists=%v (err: %v)", schemaExists, schemaErr)
		}
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyAuthMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			if _, upErr := db.Exec(ctx, databaseMigration.UpSQL); upErr != nil {
				t.Fatalf("failed re-applying migration %d: %v", databaseMigration.Version, upErr)
			}
		}

		for _, tableName := range expectedTables {
			var exists bool
			err := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'auth' AND table_name = $1
				)
			`, tableName).Scan(&exists)
			if err != nil || !exists {
				t.Fatalf("expected auth.%s to exist after re-applying migrations", tableName)
			}
		}
	})
}
