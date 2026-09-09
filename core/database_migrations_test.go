package core

import (
	"testing"
)

func TestCoreDatabaseMigrationsDefinitionsUnit(t *testing.T) {
	if len(SystemDatabaseMigrations) == 0 {
		t.Fatal("expected SystemDatabaseMigrations to have at least 1 migration")
	}

	firstDatabaseMigration := SystemDatabaseMigrations[0]
	if firstDatabaseMigration.Version != 1 {
		t.Errorf("expected version 1, got %d", firstDatabaseMigration.Version)
	}
	if firstDatabaseMigration.UpSQL == "" {
		t.Error("expected non-empty UpSQL for system migration 1")
	}
	if firstDatabaseMigration.DownSQL == "" {
		t.Error("expected non-empty DownSQL for system migration 1")
	}
}

func TestCoreDatabaseMigrationRegistrationUnit(t *testing.T) {
	initialMigrations := GetRegisteredDatabaseMigrations()
	if len(initialMigrations) == 0 {
		t.Fatal("expected non-empty default registered migrations")
	}

	customDatabaseMigration := DatabaseMigration{
		Version:     9999,
		Description: "Custom test migration",
		UpSQL:       "SELECT 1;",
		DownSQL:     "SELECT 1;",
	}

	RegisterDatabaseMigration(customDatabaseMigration)
	updatedMigrations := GetRegisteredDatabaseMigrations()
	if len(updatedMigrations) <= len(initialMigrations) {
		t.Fatalf("expected updated migrations count (%d) to be > initial (%d)", len(updatedMigrations), len(initialMigrations))
	}
	found := false
	for _, migration := range updatedMigrations {
		if migration.Version == 9999 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected custom migration 9999 to be present in GetRegisteredDatabaseMigrations")
	}
}
