package image

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Verify that image tables exist
	tablesToCheck := []string{"config", "presets", "cache_entries"}
	for _, tableName := range tablesToCheck {
		var tableExists bool
		queryErr := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'image' AND table_name = $1
			);
		`, tableName).Scan(&tableExists)
		require.NoError(t, queryErr)
		require.True(t, tableExists, "expected image.%s table to exist", tableName)
	}

	// 2. Test rollback of image migrations
	for index := len(Migrations) - 1; index >= 0; index-- {
		databaseMigration := Migrations[index]
		_, downErr := db.Exec(ctx, databaseMigration.DownSQL)
		require.NoError(t, downErr, "failed rollback of migration %d", databaseMigration.Version)
	}

	// Verify image schema was dropped
	var schemaExists bool
	schemaErr := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.schemata 
			WHERE schema_name = 'image'
		);
	`).Scan(&schemaExists)
	require.NoError(t, schemaErr)
	require.False(t, schemaExists, "expected image schema to be dropped after rollback")

	// 3. Re-apply UpSQL
	for _, databaseMigration := range Migrations {
		_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
		require.NoError(t, upErr, "failed re-applying migration %d", databaseMigration.Version)
	}
}
