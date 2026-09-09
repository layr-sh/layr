package auth

import (
	"context"
	"testing"
)

func TestAuthMigrationsExecutionIntegration(t *testing.T) {
	db, _, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	expectedTables := []string{
		"config",
		"users",
		"identities",
		"sessions",
		"passkeys",
		"otps",
	}

	for _, tableName := range expectedTables {
		var exists bool
		err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'layr_auth' AND table_name = $1
			)
		`, tableName).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to query table existence for %s: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("table layr_auth.%s does not exist after running migrations", tableName)
		}
	}
}
