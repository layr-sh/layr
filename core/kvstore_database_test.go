package core

import (
	"context"
	"testing"
	"time"
)

func TestCoreDatabaseKVStoreUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize with zero sweep interval (defaults to 60s)
	zeroKVStore := NewDatabaseKVStore(ctx, nil, 0)
	if zeroKVStore == nil {
		t.Fatal("expected non-nil DatabaseKVStore")
	}
	zeroDatabaseKVStore, ok := zeroKVStore.Driver().(*DatabaseKVStore)
	if !ok || zeroDatabaseKVStore.sweepInterval != 60*time.Second {
		t.Errorf("expected default sweep interval 60s, got %v", zeroDatabaseKVStore.sweepInterval)
	}

	// 2. Initialize with negative sweep interval (defaults to 60s)
	negativeKVStore := NewDatabaseKVStore(ctx, nil, -10*time.Second)
	negativeDatabaseKVStore, ok := negativeKVStore.Driver().(*DatabaseKVStore)
	if !ok || negativeDatabaseKVStore.sweepInterval != 60*time.Second {
		t.Errorf("expected negative sweep interval to default to 60s, got %v", negativeDatabaseKVStore.sweepInterval)
	}

	// 3. Initialize with custom sweep interval
	customKVStore := NewDatabaseKVStore(ctx, nil, 15*time.Second)
	customDatabaseKVStore, ok := customKVStore.Driver().(*DatabaseKVStore)
	if !ok || customDatabaseKVStore.sweepInterval != 15*time.Second {
		t.Errorf("expected custom sweep interval 15s, got %v", customDatabaseKVStore.sweepInterval)
	}

	// 4. Double Close safety on unstarted / nil db connection pool stores
	if err := zeroKVStore.Close(); err != nil {
		t.Fatalf("expected nil error on Close, got %v", err)
	}
	if err := zeroKVStore.Close(); err != nil {
		t.Fatalf("expected nil error on second Close, got %v", err)
	}
	_ = negativeKVStore.Close()
	_ = customKVStore.Close()

	// 5. Empty operations should return immediately without accessing the db connection pool
	emptyMGetResult, err := zeroKVStore.MGet(ctx, []string{})
	if err != nil || len(emptyMGetResult) != 0 {
		t.Fatalf("expected empty MGet to succeed without db connection pool access, got: %v", err)
	}
	if err = zeroKVStore.MSet(ctx, map[string]string{}, time.Minute); err != nil {
		t.Fatalf("expected empty MSet to succeed without db connection pool access, got: %v", err)
	}
}
