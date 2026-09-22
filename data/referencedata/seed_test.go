package referencedata

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestReferencedataLoadSeedDataUnit(t *testing.T) {
	countries, currencies, err := LoadSeedData()
	if err != nil {
		t.Fatalf("failed to load seed data: %v", err)
	}
	if len(countries) == 0 {
		t.Fatalf("expected countries to be non-empty")
	}
	if len(currencies) == 0 {
		t.Fatalf("expected currencies to be non-empty")
	}

	// Test corrupted json error branches
	restore := SetEmbeddedJSONForTesting([]byte("invalid-json"), []byte("[]"))
	if _, _, err := LoadSeedData(); err == nil {
		t.Fatalf("expected error on invalid embedded countries json")
	}
	restore()

	restore = SetEmbeddedJSONForTesting([]byte("[]"), []byte("invalid-json"))
	if _, _, err := LoadSeedData(); err == nil {
		t.Fatalf("expected error on invalid embedded currencies json")
	}
	restore()
}

func TestReferencedataSeedLoadErrorUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, core.GetRegisteredDatabaseMigrations())
	defer cleanup()

	restore := SetEmbeddedJSONForTesting([]byte("invalid"), nil)
	defer restore()

	if err := Seed(context.Background(), kernel); err == nil {
		t.Fatalf("expected error on Seed with invalid JSON")
	}
}
