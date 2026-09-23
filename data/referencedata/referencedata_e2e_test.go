package referencedata_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
	_ "layr.sh/data"
	"layr.sh/data/referencedata"
)

func TestReferencedataLifecycleE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Boot Postgres 18 testcontainer
	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr_refdata_e2e"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available for e2e test: %v", err)
		return
	}
	defer func() { _ = postgresContainer.Terminate(context.Background()) }()

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}
	defer db.Close()

	// 2. Run all registered system migrations (including migration 100 for reference_data schema)
	if err := db.RunMigrations(ctx, core.GetRegisteredDatabaseMigrations()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	kernel := core.NewTestKernel(db)

	// 3. Seed Reference Data
	if err := referencedata.Seed(ctx, kernel); err != nil {
		t.Fatalf("failed to seed reference data: %v", err)
	}

	// 4. Verify all 10 normalized reference_data tables have populated records
	tables := []string{
		"reference_data.regions",
		"reference_data.subregions",
		"reference_data.currencies",
		"reference_data.languages",
		"reference_data.timezones",
		"reference_data.countries",
		"reference_data.country_currencies",
		"reference_data.country_languages",
		"reference_data.country_timezones",
		"reference_data.country_name_translations",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var count int
			query := "SELECT COUNT(*) FROM " + table
			if err := db.QueryRow(ctx, query).Scan(&count); err != nil {
				t.Fatalf("failed to count rows in %s: %v", table, err)
			}
			if count == 0 {
				t.Fatalf("expected rows in %s, got 0", table)
			}
		})
	}

	// 5. Query relational cross-table joins for a country (e.g. US)
	var commonName, officialName, regionName, currencyCode, timezoneIdentifier string
	crossJoinQuery := `
		SELECT 
			countries.common_name,
			countries.official_name,
			COALESCE(regions.name, ''),
			COALESCE(currencies.code, ''),
			COALESCE(timezones.identifier, '')
		FROM reference_data.countries countries
		LEFT JOIN reference_data.regions regions ON countries.region_id = regions.id
		LEFT JOIN reference_data.country_currencies country_currencies ON countries.id = country_currencies.country_id
		LEFT JOIN reference_data.currencies currencies ON country_currencies.currency_id = currencies.id
		LEFT JOIN reference_data.country_timezones country_timezones ON countries.id = country_timezones.country_id
		LEFT JOIN reference_data.timezones timezones ON country_timezones.timezone_id = timezones.id
		WHERE countries.cca2 = 'US'
		LIMIT 1
	`
	if err := db.QueryRow(ctx, crossJoinQuery).Scan(&commonName, &officialName, &regionName, &currencyCode, &timezoneIdentifier); err != nil {
		t.Fatalf("failed to query cross-table joins for US: %v", err)
	}

	if commonName != "United States" || regionName != "Americas" || currencyCode != "USD" {
		t.Fatalf("unexpected cross-join query results for US: common=%s region=%s currency=%s tz=%s", commonName, regionName, currencyCode, timezoneIdentifier)
	}

	// 6. Test idempotent re-seeding
	if err := referencedata.Seed(ctx, kernel); err != nil {
		t.Fatalf("re-seeding should be idempotent, failed: %v", err)
	}
}
