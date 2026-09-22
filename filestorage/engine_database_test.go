package filestorage

import (
	"testing"

	"layr.sh/core"
)

func TestFilestorageDatabaseConstructorUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	defaultDatabaseEngine := NewDatabaseEngine(kernel, 0)
	if defaultDatabaseEngine == nil {
		t.Fatal("expected non-nil database engine")
	}
	if defaultDatabaseEngine.chunkSizeBytes != DefaultDatabaseChunkSize {
		t.Fatalf("expected chunk size %d, got %d", DefaultDatabaseChunkSize, defaultDatabaseEngine.chunkSizeBytes)
	}

	customDatabaseEngine := NewDatabaseEngine(kernel, 1024)
	if customDatabaseEngine.chunkSizeBytes != 1024 {
		t.Fatalf("expected custom chunk size 1024, got %d", customDatabaseEngine.chunkSizeBytes)
	}
}

func TestFilestorageDatabaseErrorDefinitionsUnit(t *testing.T) {
	if ErrObjectNotFound == nil || ErrBucketNotFound == nil || ErrInvalidRange == nil || ErrPayloadTooLarge == nil || ErrUnsupportedBackend == nil {
		t.Fatal("expected non-nil sentinel error definitions")
	}
}
