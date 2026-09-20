package filestorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type mockEchoDriver struct{}

func (mockEchoDriver) Upload(context.Context, Bucket, string, io.Reader, int64, string) (*Object, error) {
	return &Object{ObjectKey: "echo.txt", SizeBytes: 4}, nil
}

func (mockEchoDriver) Download(context.Context, Bucket, string, *ContentRange) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader([]byte("echo"))), 4, nil
}

func (mockEchoDriver) Delete(context.Context, Bucket, string) error {
	return nil
}

func (mockEchoDriver) Head(context.Context, Bucket, string) (*Object, error) {
	return &Object{ObjectKey: "echo.txt", SizeBytes: 4}, nil
}

func TestFilestorageEngineUnit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testBucket := Bucket{Name: "test-bucket"}

	t.Run("nil driver handling unit", func(t *testing.T) {
		nilEngine := NewEngine(nil)
		if nilEngine != nil {
			t.Fatal("expected nil Engine when driver is nil")
		}

		var uninitializedEngine *Engine
		if uninitializedEngine.Driver() != nil {
			t.Fatal("expected nil Driver from uninitialized engine")
		}

		_, uploadErr := uninitializedEngine.Upload(ctx, testBucket, "test.txt", bytes.NewReader([]byte("data")), 4, "text/plain")
		if !errors.Is(uploadErr, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", uploadErr)
		}

		_, _, downloadErr := uninitializedEngine.Download(ctx, testBucket, "test.txt", nil)
		if !errors.Is(downloadErr, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", downloadErr)
		}

		deleteErr := uninitializedEngine.Delete(ctx, testBucket, "test.txt")
		if !errors.Is(deleteErr, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", deleteErr)
		}

		_, headErr := uninitializedEngine.Head(ctx, testBucket, "test.txt")
		if !errors.Is(headErr, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", headErr)
		}

		emptyDriverEngine := &Engine{driver: nil}
		if emptyDriverEngine.Driver() != nil {
			t.Fatal("expected nil Driver from engine with nil driver")
		}
		if _, err := emptyDriverEngine.Upload(ctx, testBucket, "test.txt", nil, 0, ""); !errors.Is(err, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", err)
		}
		if _, _, err := emptyDriverEngine.Download(ctx, testBucket, "test.txt", nil); !errors.Is(err, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", err)
		}
		if err := emptyDriverEngine.Delete(ctx, testBucket, "test.txt"); !errors.Is(err, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", err)
		}
		if _, err := emptyDriverEngine.Head(ctx, testBucket, "test.txt"); !errors.Is(err, ErrEngineNotInitialized) {
			t.Fatalf("expected ErrEngineNotInitialized, got: %v", err)
		}
	})

	t.Run("driver delegation unit", func(t *testing.T) {
		echoDriver := mockEchoDriver{}
		engine := NewEngine(echoDriver)
		if engine == nil {
			t.Fatal("expected non-nil Engine")
		}
		if engine.Driver() == nil {
			t.Fatal("expected non-nil Driver")
		}

		uploadedObject, uploadErr := engine.Upload(ctx, testBucket, "echo.txt", bytes.NewReader([]byte("echo")), 4, "text/plain")
		if uploadErr != nil || uploadedObject == nil {
			t.Fatalf("unexpected upload error: %v", uploadErr)
		}

		downloadReadCloser, length, downloadErr := engine.Download(ctx, testBucket, "echo.txt", nil)
		if downloadErr != nil || length != 4 {
			t.Fatalf("unexpected download error: %v", downloadErr)
		}
		_ = downloadReadCloser.Close()

		headObject, headErr := engine.Head(ctx, testBucket, "echo.txt")
		if headErr != nil || headObject == nil {
			t.Fatalf("unexpected head error: %v", headErr)
		}

		deleteErr := engine.Delete(ctx, testBucket, "echo.txt")
		if deleteErr != nil {
			t.Fatalf("unexpected delete error: %v", deleteErr)
		}
	})

	t.Run("constructors unit", func(t *testing.T) {
		databaseEngine := NewDatabaseFileStorageEngine(nil)
		if databaseEngine == nil {
			t.Fatal("expected non-nil database file storage engine")
		}

		s3Engine := NewS3FileStorageEngine(nil)
		if s3Engine == nil {
			t.Fatal("expected non-nil s3 file storage engine")
		}
	})
}
