package auth

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestAuthMigrationsExecutionIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

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
				WHERE table_schema = 'auth' AND table_name = $1
			)
		`, tableName).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to query table existence for %s: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("table auth.%s does not exist after running migrations", tableName)
		}
	}
}
