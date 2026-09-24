package core

import (
	"context"
	"testing"
	"time"
)

func TestCoreTestSetupKernelIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	testDatabaseMigration := DatabaseMigration{
		Service:     "core",
		Version:     9999,
		Description: "test helper migration",
		UpSQL:       "CREATE TABLE IF NOT EXISTS _test_helper_table (id text PRIMARY KEY);",
		DownSQL:     "DROP TABLE IF EXISTS _test_helper_table;",
	}

	kernel, cleanup := SetupTestKernel(t, []DatabaseMigration{testDatabaseMigration})
	defer cleanup()

	if kernel == nil || kernel.DB() == nil {
		t.Fatal("expected non-nil kernel and db from SetupTestKernel")
	}

	// Broken DB test
	brokenKernel := SetupTestKernelWithBrokenDB(t, []DatabaseMigration{testDatabaseMigration})
	if brokenKernel == nil || brokenKernel.DB() == nil {
		t.Fatal("expected non-nil brokenKernel from SetupTestKernelWithBrokenDB")
	}
	// Verify broken DB returns error on query
	var scannedValue int
	if err := brokenKernel.DB().QueryRow(ctx, "SELECT 1").Scan(&scannedValue); err == nil {
		t.Fatal("expected error on broken DB query")
	}
}
