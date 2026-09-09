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
		"layr_auth.config",
		"layr_auth.users",
		"layr_auth.identities",
		"layr_auth.sessions",
		"layr_auth.passkeys",
		"layr_auth.otps",
	}

	for _, tableName := range requiredTables {
		if !strings.Contains(databaseMigration.UpSQL, tableName) {
			t.Fatalf("migration missing table definition: %s", tableName)
		}
	}
}
