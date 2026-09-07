package core

import (
	"context"
	"testing"
	"time"
)

func TestCoreDatabaseKVStoreUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize with zero sweep interval (defaults to 60s)
	databaseKVStoreZero := NewDatabaseKVStore(ctx, nil, 0)
	if databaseKVStoreZero == nil {
		t.Fatal("expected non-nil DatabaseKVStore")
	}
	if databaseKVStoreZero.sweepInterval != 60*time.Second {
		t.Errorf("expected default sweep interval 60s, got %v", databaseKVStoreZero.sweepInterval)
	}

	// 2. Initialize with negative sweep interval (defaults to 60s)
	databaseKVStoreNegative := NewDatabaseKVStore(ctx, nil, -10*time.Second)
	if databaseKVStoreNegative.sweepInterval != 60*time.Second {
		t.Errorf("expected negative sweep interval to default to 60s, got %v", databaseKVStoreNegative.sweepInterval)
	}

	// 3. Initialize with custom sweep interval
	databaseKVStoreCustom := NewDatabaseKVStore(ctx, nil, 15*time.Second)
	if databaseKVStoreCustom.sweepInterval != 15*time.Second {
		t.Errorf("expected custom sweep interval 15s, got %v", databaseKVStoreCustom.sweepInterval)
	}

	// 4. Double Close safety on unstarted / nil db connection pool stores
	if err := databaseKVStoreZero.Close(); err != nil {
		t.Fatalf("expected nil error on Close, got %v", err)
	}
	if err := databaseKVStoreZero.Close(); err != nil {
		t.Fatalf("expected nil error on second Close, got %v", err)
	}
	_ = databaseKVStoreNegative.Close()
	_ = databaseKVStoreCustom.Close()

	// 5. Empty operations should return immediately without accessing the db connection pool
	emptyMGetResult, err := databaseKVStoreZero.MGet(ctx, []string{})
	if err != nil || len(emptyMGetResult) != 0 {
		t.Fatalf("expected empty MGet to succeed without db connection pool access, got: %v", err)
	}
	if err = databaseKVStoreZero.MSet(ctx, map[string]string{}, time.Minute); err != nil {
		t.Fatalf("expected empty MSet to succeed without db connection pool access, got: %v", err)
	}
}
