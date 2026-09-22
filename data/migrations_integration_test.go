package data

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestDataMigrationsExecutionIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()

	// 1. Verify that data.config table exists
	var configTableExists bool
	queryErr := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'data' AND table_name = 'config'
		);
	`).Scan(&configTableExists)
	if queryErr != nil || !configTableExists {
		t.Fatalf("expected data.config table to exist: %v", queryErr)
	}

	// 2. Test rollback of Data migrations
	for idx := len(Migrations) - 1; idx >= 0; idx-- {
		databaseMigration := Migrations[idx]
		if _, downErr := db.Exec(ctx, databaseMigration.DownSQL); downErr != nil {
			t.Fatalf("failed rollback of migration %d: %v", databaseMigration.Version, downErr)
		}
	}

	// Verify data schema was dropped or cleaned up
	var tableCount int
	countErr := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema = 'data';
	`).Scan(&tableCount)
	if countErr != nil || tableCount != 0 {
		t.Fatalf("expected 0 tables in data after rollback, got %d (err: %v)", tableCount, countErr)
	}

	// 3. Re-apply UpSQL
	for _, databaseMigration := range Migrations {
		if _, upErr := db.Exec(ctx, databaseMigration.UpSQL); upErr != nil {
			t.Fatalf("failed re-applying migration %d: %v", databaseMigration.Version, upErr)
		}
	}
}
