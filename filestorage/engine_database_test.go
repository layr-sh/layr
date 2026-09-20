package filestorage

import (
	"context"
	"strings"
	"testing"
	"uuid"
)

func TestFilestorageDatabaseConstructorUnit(t *testing.T) {
	defaultDatabaseEngine := NewDatabaseEngine(nil, 0)
	if defaultDatabaseEngine == nil {
		t.Fatal("expected non-nil database engine")
	}
	if defaultDatabaseEngine.chunkSizeBytes != DefaultDatabaseChunkSize {
		t.Fatalf("expected chunk size %d, got %d", DefaultDatabaseChunkSize, defaultDatabaseEngine.chunkSizeBytes)
	}

	customDatabaseEngine := NewDatabaseEngine(nil, 1024)
	if customDatabaseEngine.chunkSizeBytes != 1024 {
		t.Fatalf("expected custom chunk size 1024, got %d", customDatabaseEngine.chunkSizeBytes)
	}
}

func TestFilestorageDatabaseNilDatabasePoolUnit(t *testing.T) {
	databaseEngine := NewDatabaseEngine(nil, 1024)
	ctx := context.Background()
	testBucket := Bucket{
		ID:   uuid.NewV7(),
		Name: "test-bucket",
	}

	// Upload with nil db
	_, uploadErr := databaseEngine.Upload(ctx, testBucket, "sample.txt", strings.NewReader("hello"), 5, "text/plain")
	if uploadErr == nil {
		t.Fatal("expected error on upload with nil database pool")
	}

	// Head with nil db
	_, headErr := databaseEngine.Head(ctx, testBucket, "sample.txt")
	if headErr == nil {
		t.Fatal("expected error on head with nil database pool")
	}

	// Delete with nil db
	deleteErr := databaseEngine.Delete(ctx, testBucket, "sample.txt")
	if deleteErr == nil {
		t.Fatal("expected error on delete with nil database pool")
	}

	// Download with nil db
	_, _, downloadErr := databaseEngine.Download(ctx, testBucket, "sample.txt", nil)
	if downloadErr == nil {
		t.Fatal("expected error on download with nil database pool")
	}
}

func TestFilestorageDatabaseErrorDefinitionsUnit(t *testing.T) {
	if ErrObjectNotFound == nil || ErrBucketNotFound == nil || ErrInvalidRange == nil || ErrPayloadTooLarge == nil || ErrUnsupportedBackend == nil {
		t.Fatal("expected non-nil sentinel error definitions")
	}
}
