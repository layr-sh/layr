package auth

import (
	"strings"
	"testing"
)

func TestAuthMigrationsDefinitionUnit(t *testing.T) {
	if len(Migrations) == 0 {
		t.Fatal("expected non-empty Auth Migrations")
	}

	databaseMigration := Migrations[0]
	if databaseMigration.Version != AuthDatabaseMigrationVersion {
		t.Fatalf("expected version %d, got: %d", AuthDatabaseMigrationVersion, databaseMigration.Version)
	}

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
