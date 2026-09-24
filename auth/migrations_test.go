package auth

import (
	"fmt"
	"strings"
	"testing"
)

func TestAuthMigrationsDefinitionUnit(t *testing.T) {
	if len(Migrations) == 0 {
		t.Fatal("expected at least one migration defined in Migrations")
	}

	for _, databaseMigration := range Migrations {
		t.Run(fmt.Sprintf("Version_%d", databaseMigration.Version), func(t *testing.T) {
			if databaseMigration.Service != "auth" {
				t.Fatalf("expected service 'auth', got '%s'", databaseMigration.Service)
			}
			if databaseMigration.Version <= 0 {
				t.Fatalf("expected positive migration version, got %d", databaseMigration.Version)
			}
			if databaseMigration.Description == "" {
				t.Fatalf("expected non-empty description for migration %d", databaseMigration.Version)
			}
			if databaseMigration.UpSQL == "" {
				t.Fatalf("expected non-empty UpSQL for migration version %d", databaseMigration.Version)
			}
			if databaseMigration.DownSQL == "" {
				t.Fatalf("expected non-empty DownSQL for migration version %d", databaseMigration.Version)
			}
		})
	}

	databaseMigration := Migrations[0]
	requiredTables := []string{
		"auth.config",
		"auth.users",
		"auth.identities",
		"auth.sessions",
		"auth.passkeys",
		"auth.otps",
	}

	for _, tableName := range requiredTables {
		t.Run(tableName, func(t *testing.T) {
			if !strings.Contains(databaseMigration.UpSQL, tableName) {
				t.Fatalf("migration missing table definition: %s", tableName)
			}
		})
	}
}
