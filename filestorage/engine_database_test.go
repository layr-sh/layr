package filestorage

import (
	"context"
	"io"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
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

func TestFilestorageDatabaseUploadTransactionFailuresUnit(t *testing.T) {
	for _, test := range []struct{ query, want string }{
		{"DELETE FROM file_storage.chunks", "failed to remove old object chunks"},
		{"UPDATE file_storage.objects", "failed to finalize object"},
		{"commit", "failed to commit object upload"},
	} {
		t.Run(test.want, func(t *testing.T) {
			kernel := postgresFaultKernel(t.Context(), t, func(sql string) []pgproto3.BackendMessage {
				switch {
				case strings.Contains(sql, test.query):
					return postgresFailure()
				case strings.Contains(sql, "INSERT INTO file_storage.objects"):
					return postgresResult([]uint32{pgtype.UUIDOID}, []string{fixtureObjectID})
				case strings.Contains(sql, "UPDATE file_storage.objects"):
					return fixtureObjectResult(fixtureObjectID)
				}
				return nil
			})
			databaseEngine := NewDatabaseEngine(kernel)
			bucket := Bucket{ID: uuid.MustParse(fixtureBucketID)}
			object, err := databaseEngine.Upload(context.Background(), bucket, "test.txt", strings.NewReader("test"), 4, "text/plain")
			if object != nil || err == nil || !strings.Contains(err.Error(), test.want+": ERROR: injected PostgreSQL failure") {
				t.Fatalf("expected %q, got object=%+v, error=%v", test.want, object, err)
			}
		})
	}
}

func TestFilestorageDatabaseDownloadStreamFailuresUnit(t *testing.T) {
	for _, test := range []struct {
		name     string
		response []pgproto3.BackendMessage
		want     string
	}{
		{"query", postgresFailure(), "injected PostgreSQL failure"},
		{"scan", postgresResult([]uint32{pgtype.ByteaOID}, []string{"not bytea"}), "can't scan"},
		{"stream", postgresStreamFailure(), "injected PostgreSQL failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kernel := postgresFaultKernel(t.Context(), t, func(sql string) []pgproto3.BackendMessage {
				switch {
				case strings.Contains(sql, "SELECT chunk_data"):
					return test.response
				case strings.Contains(sql, "FROM file_storage.objects"):
					return fixtureObjectResult(fixtureObjectID)
				}
				return nil
			})
			databaseEngine := NewDatabaseEngine(kernel)
			downloadReadCloser, size, err := databaseEngine.Download(context.Background(), Bucket{ID: uuid.MustParse(fixtureBucketID)}, "test.txt", nil)
			if err != nil || size != 4 {
				t.Fatalf("expected a 4-byte download stream, got size=%d, err=%v", size, err)
			}
			defer func() { _ = downloadReadCloser.Close() }()
			if data, err := io.ReadAll(downloadReadCloser); len(data) != 0 || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q from stream, got data=%q, err=%v", test.want, data, err)
			}
		})
	}
}
