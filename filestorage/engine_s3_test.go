package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"uuid"

	"github.com/jackc/pgx/v5/pgproto3"
	"layr.sh/core"
)

func TestFilestorageS3ConstructorUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	s3Engine := NewS3Engine(kernel)
	if s3Engine == nil {
		t.Fatal("expected non-nil s3 engine")
	}
}

func TestFilestorageS3MissingBackendConfigUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	s3Engine := NewS3Engine(kernel)
	ctx := context.Background()
	testBucket := Bucket{
		ID:            uuid.NewV7(),
		Name:          "s3-bucket",
		Backend:       "s3",
		BackendConfig: nil,
	}

	_, uploadErr := s3Engine.Upload(ctx, testBucket, "file.txt", strings.NewReader("data"), 4, "text/plain")
	if uploadErr == nil || !strings.Contains(uploadErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", uploadErr)
	}

	_, headErr := s3Engine.Head(ctx, testBucket, "file.txt")
	if headErr == nil || !strings.Contains(headErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", headErr)
	}

	deleteErr := s3Engine.Delete(ctx, testBucket, "file.txt")
	if deleteErr == nil || !strings.Contains(deleteErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", deleteErr)
	}

	_, _, downloadErr := s3Engine.Download(ctx, testBucket, "file.txt", nil)
	if downloadErr == nil || !strings.Contains(downloadErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", downloadErr)
	}
}

func TestFilestorageS3EncryptedSecretHandlingUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	plainSecret := "my-aws-secret-access-key-12345"
	encryptedSecret, err := kernel.CryptoKeyManager().EncryptField([]byte(plainSecret))
	if err != nil {
		t.Fatalf("failed to encrypt secret: %v", err)
	}

	cryptoS3Engine := NewS3Engine(kernel)
	testBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "s3-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          "http://localhost:4566",
			"bucket":            "test-bucket",
			"region":            "us-east-1",
			"access_key_id":     "TESTKEY",
			"secret_access_key": encryptedSecret,
		},
	}

	s3UpstreamConfiguration, parseErr := cryptoS3Engine.parseUpstreamConfiguration(testBucket)
	if parseErr != nil {
		t.Fatalf("failed to parse upstream config: %v", parseErr)
	}
	if s3UpstreamConfiguration.SecretAccessKey != plainSecret {
		t.Fatalf("expected decrypted secret %q, got %q", plainSecret, s3UpstreamConfiguration.SecretAccessKey)
	}

	// Invalid encrypted secret fails to decrypt
	corruptBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "corrupt-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"secret_access_key": "enc:v1:aes256gcm:not-valid-hex-or-b64",
		},
	}
	_, decryptErr := cryptoS3Engine.parseUpstreamConfiguration(corruptBucket)
	if decryptErr == nil || !strings.Contains(decryptErr.Error(), "failed to decrypt s3 secret_access_key") {
		t.Fatalf("expected failed to decrypt error, got: %v", decryptErr)
	}

	// Defaults fallback for bucket name and region
	defaultsBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "fallback-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"access_key_id":     "TESTKEY",
			"secret_access_key": "raw-secret",
		},
	}
	defaultsS3UpstreamConfiguration, defaultsErr := cryptoS3Engine.parseUpstreamConfiguration(defaultsBucket)
	if defaultsErr != nil {
		t.Fatalf("failed to parse fallback bucket config: %v", defaultsErr)
	}
	if defaultsS3UpstreamConfiguration.BucketName != "fallback-bucket" {
		t.Fatalf("expected fallback bucket name 'fallback-bucket', got %q", defaultsS3UpstreamConfiguration.BucketName)
	}
	if defaultsS3UpstreamConfiguration.Region != "us-east-1" {
		t.Fatalf("expected fallback region 'us-east-1', got %q", defaultsS3UpstreamConfiguration.Region)
	}
}

func TestFilestorageS3OperationsUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	s3Engine := NewS3Engine(kernel)
	ctx := context.Background()

	// Test getClient with empty Endpoint to cover that branch
	noEndpointClient := s3Engine.getClient(&S3UpstreamConfiguration{
		Region:          "us-east-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	})
	if noEndpointClient == nil {
		t.Fatal("expected non-nil s3 client")
	}

	storage := make(map[string][]byte)
	var failMethod string

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if failMethod != "" && request.Method == failMethod {
			http.Error(responseWriter, "internal mock error", http.StatusInternalServerError)
			return
		}

		key := strings.TrimPrefix(request.URL.Path, "/test-bucket/")
		switch request.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(request.Body)
			storage[key] = body
			responseWriter.Header().Set("ETag", "\"mock-etag-123\"")
			responseWriter.WriteHeader(http.StatusOK)
		case http.MethodHead:
			data, exists := storage[key]
			if !exists {
				http.NotFound(responseWriter, request)
				return
			}
			responseWriter.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			responseWriter.Header().Set("Content-Type", "text/plain")
			responseWriter.Header().Set("ETag", "\"mock-etag-123\"")
			responseWriter.WriteHeader(http.StatusOK)
		case http.MethodGet:
			data, exists := storage[key]
			if !exists {
				http.NotFound(responseWriter, request)
				return
			}
			rangeHdr := request.Header.Get("Range")
			if rangeHdr != "" {
				responseWriter.Header().Set("Content-Range", "bytes 0-4/11")
				responseWriter.WriteHeader(http.StatusPartialContent)
				_, _ = responseWriter.Write(data[:5])
				return
			}
			responseWriter.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(data)
		case http.MethodDelete:
			delete(storage, key)
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			http.Error(responseWriter, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	testBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "test-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          server.URL,
			"bucket":            "test-bucket",
			"region":            "us-east-1",
			"access_key_id":     "MOCKKEY",
			"secret_access_key": "MOCKSECRET",
		},
	}

	t.Run("upstream succeeds but metadata fails", func(t *testing.T) {
		kernel := postgresFaultKernel(ctx, t, func(sql string) []pgproto3.BackendMessage {
			if strings.Contains(sql, "file_storage.objects") {
				return postgresFailure()
			}
			return nil
		})
		metadataS3Engine := NewS3Engine(kernel)
		const key = "metadata-failure.txt"
		if _, err := metadataS3Engine.Upload(ctx, testBucket, key, strings.NewReader("test"), 4, "text/plain"); err == nil || !strings.Contains(err.Error(), "failed to register upstream object metadata: ERROR: injected PostgreSQL failure") {
			t.Fatalf("expected metadata registration error, got %v", err)
		}
		if err := metadataS3Engine.Delete(ctx, testBucket, key); err == nil || !strings.Contains(err.Error(), "failed to delete upstream object metadata: ERROR: injected PostgreSQL failure") {
			t.Fatalf("expected metadata deletion error, got %v", err)
		}
	})

	// 1. Upload success with custom and empty content type
	uploadedObject, err := s3Engine.Upload(ctx, testBucket, "file.txt", strings.NewReader("hello world"), 11, "text/plain")
	if err != nil || uploadedObject.ChecksumSHA256 != "mock-etag-123" {
		t.Fatalf("upload failed: %v", err)
	}

	defaultObject, err := s3Engine.Upload(ctx, testBucket, "default.bin", strings.NewReader("binary"), 6, "")
	if err != nil || defaultObject.ContentType != "application/octet-stream" {
		t.Fatalf("upload with default content type failed: %v", err)
	}

	// 2. Upload failure
	failMethod = http.MethodPut
	_, err = s3Engine.Upload(ctx, testBucket, "err.txt", strings.NewReader("err"), 3, "text/plain")
	if err == nil {
		t.Fatal("expected upload error")
	}
	failMethod = ""

	// 3. Head success
	headedObject, err := s3Engine.Head(ctx, testBucket, "file.txt")
	if err != nil || headedObject.SizeBytes != 11 {
		t.Fatalf("head failed: %v", err)
	}

	// 4. Head not found
	_, err = s3Engine.Head(ctx, testBucket, "nonexistent.txt")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got: %v", err)
	}

	// 5. Head failure
	failMethod = http.MethodHead
	_, err = s3Engine.Head(ctx, testBucket, "file.txt")
	if err == nil {
		t.Fatal("expected head error")
	}
	failMethod = ""

	// 6. Download full success
	downloadReadCloser, lenDownloaded, err := s3Engine.Download(ctx, testBucket, "file.txt", nil)
	if err != nil || lenDownloaded != 11 {
		t.Fatalf("download failed: %v", err)
	}
	data, _ := io.ReadAll(downloadReadCloser)
	_ = downloadReadCloser.Close()
	if string(data) != "hello world" {
		t.Fatalf("download content mismatch: %q", string(data))
	}

	// 7. Download partial range
	partialContentRange := &ContentRange{Start: 0, End: 4, Total: 11, IsPartial: true}
	partialDownloadReadCloser, _, err := s3Engine.Download(ctx, testBucket, "file.txt", partialContentRange)
	if err != nil {
		t.Fatalf("partial download failed: %v", err)
	}
	partData, _ := io.ReadAll(partialDownloadReadCloser)
	_ = partialDownloadReadCloser.Close()
	if string(partData) != "hello" {
		t.Fatalf("partial content mismatch: %q", string(partData))
	}

	// 8. Download not found
	_, _, err = s3Engine.Download(ctx, testBucket, "nonexistent.txt", nil)
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got: %v", err)
	}

	// 9. Download failure
	failMethod = http.MethodGet
	_, _, err = s3Engine.Download(ctx, testBucket, "file.txt", nil)
	if err == nil {
		t.Fatal("expected download error")
	}
	failMethod = ""

	// 10. Delete success
	deleteErr := s3Engine.Delete(ctx, testBucket, "file.txt")
	if deleteErr != nil {
		t.Fatalf("delete failed: %v", deleteErr)
	}

	// 11. Delete failure
	failMethod = http.MethodDelete
	failDeleteErr := s3Engine.Delete(ctx, testBucket, "default.bin")
	if failDeleteErr == nil {
		t.Fatal("expected delete error")
	}
	failMethod = ""

	// 12. Nil headers response coverage
	noHeadersServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer noHeadersServer.Close()

	noHeadersBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "no-hdr",
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          noHeadersServer.URL,
			"bucket":            "no-hdr",
			"region":            "us-east-1",
			"access_key_id":     "K",
			"secret_access_key": "S",
		},
	}
	noEtagObject, noEtagErr := s3Engine.Upload(ctx, noHeadersBucket, "noetag.txt", strings.NewReader("data"), 4, "text/plain")
	if noEtagErr != nil || noEtagObject.ChecksumSHA256 != "" {
		t.Fatalf("upload no etag failed: %v", noEtagErr)
	}
	noHdrObject, noHdrErr := s3Engine.Head(ctx, noHeadersBucket, "nohdr.txt")
	if noHdrErr != nil || noHdrObject.SizeBytes != 0 {
		t.Fatalf("head no hdr failed: %v", noHdrErr)
	}
	noLenReadCloser, lenZero, noLenErr := s3Engine.Download(ctx, noHeadersBucket, "nolen.txt", nil)
	if noLenErr != nil || lenZero != 0 {
		t.Fatalf("download no len failed: %v", noLenErr)
	}
	_ = noLenReadCloser.Close()
}

// A WriterTo source sees the staging file via io.Copy. Close it after a
// successful copy to simulate a file that became unusable before the rewind.
type closeAfterCopyReader struct{ *strings.Reader }

func (reader closeAfterCopyReader) WriteTo(writer io.Writer) (int64, error) {
	written, err := reader.Reader.WriteTo(writer)
	if err != nil {
		return written, fmt.Errorf("copy to staging writer: %w", err)
	}
	closer, ok := writer.(io.Closer)
	if !ok {
		return written, errors.New("staging writer is not closable")
	}
	if err := closer.Close(); err != nil {
		return written, fmt.Errorf("close staging writer: %w", err)
	}
	return written, nil
}

func TestFilestorageS3UploadStagingErrorsUnit(t *testing.T) {
	s3Engine := NewS3Engine(core.NewTestKernel(nil))
	bucket := Bucket{Name: "test-bucket", MaxFileSizeBytes: 4, BackendConfig: map[string]any{
		"bucket": "test-bucket", "region": "us-east-1", "access_key_id": "test-key", "secret_access_key": "test-secret",
	}}
	readErr := errors.New("source read failed")
	for _, test := range []struct {
		name     string
		reader   io.Reader
		causeErr error
		want     string
	}{
		{"temp directory missing", strings.NewReader("test"), os.ErrNotExist, "failed to stage upload"},
		{"copy fails", iotest.ErrReader(readErr), readErr, "failed to stage upload"},
		{"size probe fails", io.MultiReader(strings.NewReader("test"), iotest.ErrReader(readErr)), readErr, "failed reading upload"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if errors.Is(test.causeErr, os.ErrNotExist) {
				missing := filepath.Join(t.TempDir(), "missing")
				t.Setenv("TMPDIR", missing)
				t.Setenv("TMP", missing)
				t.Setenv("TEMP", missing)
			}
			object, err := s3Engine.Upload(context.Background(), bucket, "test.txt", test.reader, -1, "text/plain")
			if object != nil || !errors.Is(err, test.causeErr) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q wrapping %v, got object=%+v, err=%v", test.want, test.causeErr, object, err)
			}
		})
	}
	t.Run("rewind fails", func(t *testing.T) {
		bucket.MaxFileSizeBytes = 0
		object, err := s3Engine.Upload(t.Context(), bucket, "test.txt", closeAfterCopyReader{strings.NewReader("test")}, -1, "text/plain")
		if object != nil || !errors.Is(err, os.ErrClosed) || !strings.Contains(err.Error(), "failed to rewind upload") {
			t.Fatalf("expected closed staging file error, got object=%+v, err=%v", object, err)
		}
	})
}
