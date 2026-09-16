package referencedata_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
	_ "layr.sh/data"
	"layr.sh/data/referencedata"
)

func startReferenceDataTestContainer(t *testing.T) (*core.DatabasePool, string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr_refdata_test"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		cancel()
		t.Skipf("docker not available: %v", err)
		return nil, "", nil
	}

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		cancel()
		t.Fatalf("failed to connect pool: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
		cancel()
	}

	return db, databaseURL, cleanup
}

func TestReferencedataSeedLifecycleIntegration(t *testing.T) {
	db, _, cleanup := startReferenceDataTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Apply migrations
	if err := db.RunMigrations(ctx, core.GetRegisteredDatabaseMigrations()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// 1. Initial Seeding
	if err := referencedata.Seed(ctx, db); err != nil {
		t.Fatalf("Seed failed: %v", err)
	}

	// 2. Assert counts across all 16 tables
	tablesToVerify := []string{
		"reference_data.regions",
		"reference_data.subregions",
		"reference_data.countries",
		"reference_data.currencies",
		"reference_data.languages",
		"reference_data.timezones",
		"reference_data.country_currencies",
		"reference_data.country_languages",
		"reference_data.country_timezones",
		"reference_data.country_borders",
		"reference_data.country_name_translations",
		"reference_data.country_native_names",
		"reference_data.country_demonyms",
		"reference_data.country_capitals",
		"reference_data.country_tlds",
		"reference_data.country_alt_spellings",
	}

	for _, tableName := range tablesToVerify {
		var count int
		err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query count for %s: %v", tableName, err)
		}
		if count == 0 {
			t.Fatalf("expected rows in %s, got 0", tableName)
		}
	}

	// 3. Idempotent Second Seed (should not duplicate or fail)
	if err := referencedata.Seed(ctx, db); err != nil {
		t.Fatalf("idempotent Seed failed: %v", err)
	}

	// 4. Test FK Cascading deletion
	var countryID uuid.UUID
	err := db.QueryRow(ctx, "SELECT id FROM reference_data.countries WHERE cca2 = 'US'").Scan(&countryID)
	if err != nil {
		t.Fatalf("failed to query US country ID: %v", err)
	}

	_, err = db.Exec(ctx, "DELETE FROM reference_data.countries WHERE id = $1", countryID)
	if err != nil {
		t.Fatalf("failed to delete US country: %v", err)
	}

	var orphanedCurrencies int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM reference_data.country_currencies WHERE country_id = $1", countryID).Scan(&orphanedCurrencies)
	if orphanedCurrencies != 0 {
		t.Fatalf("expected 0 orphaned country_currencies, got %d", orphanedCurrencies)
	}

	// Rollback migration
	if err := db.MigrateDown(ctx, core.GetRegisteredDatabaseMigrations(), 1); err != nil {
		t.Fatalf("failed to rollback migration: %v", err)
	}
}

func TestReferencedataSeedBranchesAndErrorsIntegration(t *testing.T) {
	db, databaseURL, cleanup := startReferenceDataTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Apply migrations
	if err := db.RunMigrations(ctx, core.GetRegisteredDatabaseMigrations()); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// 1. Seed with custom edge cases: country with no region, subregion with no region, etc.
	isIndependent := true
	customCountries := []referencedata.Country{
		{
			Name: referencedata.CountryName{
				Common:   "Testland",
				Official: "Republic of Testland",
				Native: map[string]referencedata.CountryNativeDetail{
					"tst": {Common: "Test", Official: "Official Test"},
					"":    {Common: "Empty", Official: "Empty"},
				},
			},
			CCA2:        "TL",
			CCA3:        "TLS",
			Independent: &isIndependent,
			Region:      "Europe",
			Subregion:   "Independent Zone",
			Currencies: map[string]referencedata.CurrencyDetail{
				"TLS": {Name: "Testland Dollar", Symbol: "T$"},
				"EXT": {Name: "Extra Currency", Symbol: "E$"},
				"":    {Name: "Empty"},
			},
			Languages: map[string]string{
				"tst": "Testish",
				"":    "Empty",
			},
			Translations: map[string]referencedata.TranslationDetail{
				"fra": {Common: "Testlandie", Official: "Republique de Testlandie"},
				"":    {Common: "Empty", Official: "Empty"},
			},
			Demonyms: map[string]referencedata.DemonymDetail{
				"eng": {F: "Testlander", M: "Testlander"},
				"":    {F: "Empty", M: "Empty"},
			},
			Capital:      []string{"Test City", ""},
			TLD:          []string{".tl", ""},
			AltSpellings: []string{"TLS", ""},
			Borders:      []string{"CAN"},
			Timezones:    []string{"UTC+00:00", ""},
			LatLng:       []float64{10.0, 20.0},
		},
		{
			Name: referencedata.CountryName{
				Common:   "NoCodeLand",
				Official: "No Code Land",
			},
			CCA2:      "",
			CCA3:      "",
			Region:    "",
			Subregion: "Standalone Subregion",
			LatLng:    []float64{},
		},
		{
			Name: referencedata.CountryName{
				Common:   "Canada",
				Official: "Canada",
			},
			CCA2:    "CA",
			CCA3:    "CAN",
			Borders: []string{"TLS"},
		},
	}

	numericCode999 := 999
	minorUnit2 := 2
	numericCode978 := 978
	customCurrencies := []referencedata.Currency{
		{
			AlphabeticCode: "",
		},
		{
			AlphabeticCode: "TLS",
			Name:           "Testland Dollar",
			NumericCode:    &numericCode999,
			MinorUnit:      &minorUnit2,
		},
		{
			AlphabeticCode: "EUR",
			Name:           "Euro",
			NumericCode:    &numericCode978,
			MinorUnit:      &minorUnit2,
		},
	}

	if err := referencedata.SeedWithData(ctx, db, customCountries, customCurrencies); err != nil {
		t.Fatalf("failed to seed custom data: %v", err)
	}

	// 2. Closed DB pool error branches (db.Begin error)
	closedDB, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create pool to close: %v", err)
	}
	closedDB.Close()
	if err := referencedata.SeedWithData(ctx, closedDB, customCountries, customCurrencies); err == nil {
		t.Fatalf("expected error on closed pool")
	}

	// 3. Constraint-based error injection across all tables
	tableConstraints := []struct {
		table      string
		constraint string
	}{
		{"reference_data.regions", "CHECK (name != 'Europe')"},
		{"reference_data.subregions", "CHECK (name != 'Independent Zone')"},
		{"reference_data.subregions", "CHECK (name != 'Standalone Subregion')"},
		{"reference_data.currencies", "CHECK (code != 'TLS')"},
		{"reference_data.currencies", "CHECK (code != 'EXT')"},
		{"reference_data.languages", "CHECK (code != 'tst')"},
		{"reference_data.timezones", "CHECK (identifier != 'UTC+00:00')"},
		{"reference_data.countries", "CHECK (cca2 != 'TL')"},
		{"reference_data.country_currencies", "CHECK (symbol != 'T$')"},
		{"reference_data.country_languages", "CHECK (false)"},
		{"reference_data.country_timezones", "CHECK (false)"},
		{"reference_data.country_borders", "CHECK (false)"},
		{"reference_data.country_name_translations", "CHECK (common_name != 'Testlandie')"},
		{"reference_data.country_native_names", "CHECK (common_name != 'Test')"},
		{"reference_data.country_demonyms", "CHECK (female != 'Testlander')"},
		{"reference_data.country_capitals", "CHECK (name != 'Test City')"},
		{"reference_data.country_tlds", "CHECK (tld != '.tl')"},
		{"reference_data.country_alt_spellings", "CHECK (spelling != 'TLS')"},
	}

	truncateTables := `
		TRUNCATE reference_data.regions, reference_data.subregions, reference_data.currencies, 
		         reference_data.languages, reference_data.timezones, reference_data.countries CASCADE
	`

	for constraintIndex, testCase := range tableConstraints {
		_, _ = db.Exec(ctx, truncateTables)
		constraintName := "fail_constraint"
		alterAdd := "ALTER TABLE " + testCase.table + " ADD CONSTRAINT " + constraintName + " " + testCase.constraint
		if _, err := db.Exec(ctx, alterAdd); err != nil {
			t.Fatalf("failed to add constraint on %s (test %d): %v", testCase.table, constraintIndex, err)
		}

		if err := referencedata.SeedWithData(ctx, db, customCountries, customCurrencies); err == nil {
			t.Fatalf("expected error on %s constraint (%s)", testCase.table, testCase.constraint)
		}

		alterDrop := "ALTER TABLE " + testCase.table + " DROP CONSTRAINT " + constraintName
		if _, err := db.Exec(ctx, alterDrop); err != nil {
			t.Fatalf("failed to drop constraint on %s: %v", testCase.table, err)
		}
		_, _ = db.Exec(ctx, truncateTables)
	}
}

func TestReferencedataSeedCountryCurrencyErrorIntegration(t *testing.T) {
	db, _, cleanup := startReferenceDataTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Apply migrations
	if err := db.RunMigrations(ctx, core.GetRegisteredDatabaseMigrations()); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// Test country currency insert error (e.g. varchar(3) overflow on currency code)
	overflowCountry := []referencedata.Country{
		{
			CCA2: "OF",
			CCA3: "OFL",
			Name: referencedata.CountryName{Common: "Overflow", Official: "Overflow"},
			Currencies: map[string]referencedata.CurrencyDetail{
				"OVERFLOW": {Name: "Too Long"},
			},
		},
	}
	if err := referencedata.SeedWithData(ctx, db, overflowCountry, nil); err == nil {
		t.Fatalf("expected error on currency code overflow")
	}
}
