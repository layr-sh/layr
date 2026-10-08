package filestorage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	tablesToCheck := []string{"config", "buckets", "objects", "chunks", "s3_credentials", "multipart_uploads", "multipart_parts"}

	// 1. Verify that file_storage tables exist
	t.Run("VerifyFileStorageTablesExist", func(t *testing.T) {
		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'file_storage' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected file_storage.%s to exist", tableName)
		}
	})

	// 2. Test rollback of file storage migrations
	t.Run("RollbackFileStorageMigrations", func(t *testing.T) {
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
				WHERE schema_name = 'file_storage'
			);
		`).Scan(&schemaExists)
		require.NoError(t, err)
		require.False(t, schemaExists)
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyFileStorageMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
			require.NoError(t, upErr)
		}

		for _, tableName := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'file_storage' AND table_name = $1
				);
			`, tableName).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected file_storage.%s to exist after reapply", tableName)
		}
	})
}
