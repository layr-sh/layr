package filestorage

import (
	"context"
	"testing"
)

func TestFilestorageMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := setupTestFileStorageDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// 1. Verify that file_storage tables exist
	tablesToCheck := []string{"config", "buckets", "objects", "chunks", "s3_credentials", "multipart_uploads", "multipart_parts"}
	for _, tableName := range tablesToCheck {
		var tableExists bool
		queryErr := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'file_storage' AND table_name = $1
			);
		`, tableName).Scan(&tableExists)
		if queryErr != nil || !tableExists {
			t.Fatalf("expected file_storage.%s table to exist: %v", tableName, queryErr)
		}
	}

	// 2. Test rollback of file storage migrations
	for index := len(Migrations) - 1; index >= 0; index-- {
		databaseMigration := Migrations[index]
		if _, downErr := db.Exec(ctx, databaseMigration.DownSQL); downErr != nil {
			t.Fatalf("failed rollback of migration %d: %v", databaseMigration.Version, downErr)
		}
	}

	// Verify file_storage schema was dropped
	var schemaExists bool
	schemaErr := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.schemata 
			WHERE schema_name = 'file_storage'
		);
	`).Scan(&schemaExists)
	if schemaErr != nil || schemaExists {
		t.Fatalf("expected file_storage schema to be dropped after rollback, got exists=%v (err: %v)", schemaExists, schemaErr)
	}

	// 3. Re-apply UpSQL
	for _, databaseMigration := range Migrations {
		if _, upErr := db.Exec(ctx, databaseMigration.UpSQL); upErr != nil {
			t.Fatalf("failed re-applying migration %d: %v", databaseMigration.Version, upErr)
		}
	}
}
