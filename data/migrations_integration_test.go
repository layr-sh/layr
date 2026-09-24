package data

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestDataMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// 1. Verify that data.config and reference_data tables exist
	t.Run("VerifyDataTablesExist", func(t *testing.T) {
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

		var countriesTableExists bool
		countriesErr := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'reference_data' AND table_name = 'countries'
			);
		`).Scan(&countriesTableExists)
		if countriesErr != nil || !countriesTableExists {
			t.Fatalf("expected reference_data.countries table to exist: %v", countriesErr)
		}
	})

	// 2. Test rollback of data migrations
	t.Run("RollbackDataMigrations", func(t *testing.T) {
		for index := len(Migrations) - 1; index >= 0; index-- {
			databaseMigration := Migrations[index]
			if _, downErr := db.Exec(ctx, databaseMigration.DownSQL); downErr != nil {
				t.Fatalf("failed rollback of migration %d: %v", databaseMigration.Version, downErr)
			}
		}

		var dataSchemaExists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.schemata 
				WHERE schema_name = 'data'
			);
		`).Scan(&dataSchemaExists); err != nil || dataSchemaExists {
			t.Fatalf("expected data schema to be dropped after rollback, got exists=%v (err: %v)", dataSchemaExists, err)
		}

		var refSchemaExists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.schemata 
				WHERE schema_name = 'reference_data'
			);
		`).Scan(&refSchemaExists); err != nil || refSchemaExists {
			t.Fatalf("expected reference_data schema to be dropped after rollback, got exists=%v (err: %v)", refSchemaExists, err)
		}
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyDataMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			if _, upErr := db.Exec(ctx, databaseMigration.UpSQL); upErr != nil {
				t.Fatalf("failed re-applying migration %d: %v", databaseMigration.Version, upErr)
			}
		}

		var configTableExists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'data' AND table_name = 'config'
			);
		`).Scan(&configTableExists); err != nil || !configTableExists {
			t.Fatalf("expected data.config to exist after re-applying migrations")
		}
	})
}
