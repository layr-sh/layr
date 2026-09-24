package core

import (
	"fmt"
	"testing"
)

func TestCoreDatabaseMigrationsDefinitionsUnit(t *testing.T) {
	if len(SystemDatabaseMigrations) == 0 {
		t.Fatal("expected SystemDatabaseMigrations to have at least 1 migration")
	}

	firstDatabaseMigration := SystemDatabaseMigrations[0]
	if firstDatabaseMigration.Service != "core" {
		t.Errorf("expected service 'core', got '%s'", firstDatabaseMigration.Service)
	}
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
		Service:     "core",
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
		if migration.Service == "core" && migration.Version == 9999 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected custom migration core:9999 to be present in GetRegisteredDatabaseMigrations")
	}

	// Test distinct services with same version both persist
	serviceADatabaseMigration := DatabaseMigration{
		Service:     "service_a",
		Version:     8888,
		Description: "Service A migration",
		UpSQL:       "SELECT 1;",
		DownSQL:     "SELECT 1;",
	}
	serviceBDatabaseMigration := DatabaseMigration{
		Service:     "service_b",
		Version:     8888,
		Description: "Service B migration",
		UpSQL:       "SELECT 1;",
		DownSQL:     "SELECT 1;",
	}
	RegisterDatabaseMigration(serviceADatabaseMigration)
	RegisterDatabaseMigration(serviceBDatabaseMigration)

	multiMigrations := GetRegisteredDatabaseMigrations()
	foundA, foundB := false, false
	for _, migration := range multiMigrations {
		if migration.Service == "service_a" && migration.Version == 8888 {
			foundA = true
		}
		if migration.Service == "service_b" && migration.Version == 8888 {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected both service_a:8888 (found=%v) and service_b:8888 (found=%v) in GetRegisteredDatabaseMigrations", foundA, foundB)
	}

	// Test registering with empty service defaults to core
	defaultSvcDatabaseMigration := DatabaseMigration{
		Version:     7777,
		Description: "Empty service default",
		UpSQL:       "SELECT 1;",
		DownSQL:     "SELECT 1;",
	}
	RegisterDatabaseMigration(defaultSvcDatabaseMigration)
	foundDefault := false
	for _, migration := range GetRegisteredDatabaseMigrations() {
		if migration.Service == "core" && migration.Version == 7777 {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Fatal("expected migration with empty service to default to core:7777")
	}
}

func TestCoreDatabaseMigrationSortingUnit(t *testing.T) {
	migrations := []DatabaseMigration{
		{Service: "zebra", Version: 1},
		{Service: "alpha", Version: 2},
		{Service: "core", Version: 2},
		{Service: "alpha", Version: 1},
		{Service: "core", Version: 1},
	}
	sortDatabaseMigrations(migrations)

	expected := []struct {
		Service string
		Version int
	}{
		{"core", 1},
		{"core", 2},
		{"alpha", 1},
		{"alpha", 2},
		{"zebra", 1},
	}

	for idx, tc := range expected {
		t.Run(fmt.Sprintf("Index_%d_%s_%d", idx, tc.Service, tc.Version), func(t *testing.T) {
			if migrations[idx].Service != tc.Service || migrations[idx].Version != tc.Version {
				t.Fatalf("at index %d: expected %s:%d, got %s:%d", idx, tc.Service, tc.Version, migrations[idx].Service, migrations[idx].Version)
			}
		})
	}
}
