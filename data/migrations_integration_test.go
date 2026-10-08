package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestDataMigrationsExecutionIntegration(t *testing.T) {
	db, cleanup := core.SetupTestDB(t, Migrations)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	tablesToCheck := []struct {
		schema string
		table  string
	}{
		{"data", "config"},
		{"reference_data", "regions"},
		{"reference_data", "subregions"},
		{"reference_data", "countries"},
		{"reference_data", "currencies"},
		{"reference_data", "languages"},
		{"reference_data", "timezones"},
		{"reference_data", "country_currencies"},
		{"reference_data", "country_languages"},
		{"reference_data", "country_timezones"},
		{"reference_data", "country_borders"},
		{"reference_data", "country_name_translations"},
		{"reference_data", "country_native_names"},
		{"reference_data", "country_demonyms"},
		{"reference_data", "country_capitals"},
		{"reference_data", "country_tlds"},
		{"reference_data", "country_alt_spellings"},
	}
	schemasToCheck := []string{"data", "reference_data"}

	// 1. Verify that data tables exist
	t.Run("VerifyDataTablesExist", func(t *testing.T) {
		for _, target := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = $1 AND table_name = $2
				);
			`, target.schema, target.table).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected %s.%s to exist", target.schema, target.table)
		}
	})

	// 2. Test rollback of data migrations
	t.Run("RollbackDataMigrations", func(t *testing.T) {
		for idx := len(Migrations) - 1; idx >= 0; idx-- {
			databaseMigration := Migrations[idx]
			_, downErr := db.Exec(ctx, databaseMigration.DownSQL)
			require.NoError(t, downErr)
		}

		// Verify schemas are dropped
		for _, schemaName := range schemasToCheck {
			var schemaExists bool
			err := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.schemata 
					WHERE schema_name = $1
				);
			`, schemaName).Scan(&schemaExists)
			require.NoError(t, err)
			require.False(t, schemaExists)
		}
	})

	// 3. Re-apply UpSQL
	t.Run("ReapplyDataMigrations", func(t *testing.T) {
		for _, databaseMigration := range Migrations {
			_, upErr := db.Exec(ctx, databaseMigration.UpSQL)
			require.NoError(t, upErr)
		}

		for _, target := range tablesToCheck {
			var tableExists bool
			queryErr := db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = $1 AND table_name = $2
				);
			`, target.schema, target.table).Scan(&tableExists)
			require.NoError(t, queryErr)
			require.True(t, tableExists, "expected %s.%s to exist after reapply", target.schema, target.table)
		}
	})
}
