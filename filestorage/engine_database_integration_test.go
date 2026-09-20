package filestorage

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"uuid"
)

func TestFilestorageDatabaseChunkStreamingIntegration(t *testing.T) {
	db, cleanup := setupTestFileStorageDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Use small 64KB chunk size to test multi-chunk slicing and boundary traversal
	const testChunkSize = 65536
	databaseEngine := NewDatabaseEngine(db, testChunkSize)

	bucketID := uuid.NewV7()
	_, bucketErr := db.Exec(ctx, `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, max_file_size_bytes)
		VALUES ($1, 'stream-bucket', true, 'database', 10485760);
	`, bucketID)
	if bucketErr != nil {
		t.Fatalf("failed to create test bucket: %v", bucketErr)
	}

	testBucket := Bucket{
		ID:               bucketID,
		Name:             "stream-bucket",
		Backend:          "database",
		MaxFileSizeBytes: 10485760,
	}

	// 1. Upload multi-chunk file (150KB -> spans 3 chunks: 64KB + 64KB + 22KB)
	testPayload := make([]byte, 153600)
	_, _ = rand.Read(testPayload)
	const testKey = "uploads/binary.dat"

	uploadedObject, uploadErr := databaseEngine.Upload(ctx, testBucket, testKey, bytes.NewReader(testPayload), int64(len(testPayload)), "application/octet-stream")
	if uploadErr != nil {
		t.Fatalf("failed to upload multi-chunk object: %v", uploadErr)
	}
	if uploadedObject.SizeBytes != int64(len(testPayload)) {
		t.Fatalf("expected size %d, got %d", len(testPayload), uploadedObject.SizeBytes)
	}

	// Verify chunk count in DB
	var chunkCount int
	countErr := db.QueryRow(ctx, "SELECT COUNT(*) FROM file_storage.chunks WHERE object_id = $1", uploadedObject.ID).Scan(&chunkCount)
	if countErr != nil || chunkCount != 3 {
		t.Fatalf("expected 3 chunks in database, got %d (err: %v)", chunkCount, countErr)
	}

	// 1.1 Test overwrite and default content type on a separate key
	overwriteKey := "updates/overwrite.txt"
	initialPayload := []byte("Initial content")
	_, initUploadErr := databaseEngine.Upload(ctx, testBucket, overwriteKey, bytes.NewReader(initialPayload), int64(len(initialPayload)), "text/plain")
	if initUploadErr != nil {
		t.Fatalf("failed initial upload: %v", initUploadErr)
	}
	updatedPayload := []byte("Overwritten content")
	overwrittenObject, overwriteErr := databaseEngine.Upload(ctx, testBucket, overwriteKey, bytes.NewReader(updatedPayload), int64(len(updatedPayload)), "")
	if overwriteErr != nil {
		t.Fatalf("failed to overwrite object: %v", overwriteErr)
	}
	if overwrittenObject.ContentType != "application/octet-stream" {
		t.Fatalf("expected default content type, got %q", overwrittenObject.ContentType)
	}
	_ = databaseEngine.Delete(ctx, testBucket, overwriteKey)

	// 2. Head object
	headObject, headErr := databaseEngine.Head(ctx, testBucket, testKey)
	if headErr != nil {
		t.Fatalf("failed to head object: %v", headErr)
	}
	if headObject.ID != uploadedObject.ID || headObject.SizeBytes != int64(len(testPayload)) {
		t.Fatalf("head metadata mismatch: %+v vs %+v", headObject, uploadedObject)
	}

	// 2.1 Head non-existent object
	_, missingHeadErr := databaseEngine.Head(ctx, testBucket, "non-existent-key")
	if missingHeadErr == nil || !strings.Contains(missingHeadErr.Error(), "object not found") {
		t.Fatalf("expected ErrObjectNotFound on head non-existent, got: %v", missingHeadErr)
	}

	// 3. Download entire object
	downloadReadCloser, length, downloadErr := databaseEngine.Download(ctx, testBucket, testKey, nil)
	if downloadErr != nil {
		t.Fatalf("failed to download full object: %v", downloadErr)
	}
	defer func() {
		_ = downloadReadCloser.Close()
	}()

	downloadedBytes, err := io.ReadAll(downloadReadCloser)
	if err != nil {
		t.Fatalf("failed reading full download: %v", err)
	}
	if !bytes.Equal(downloadedBytes, testPayload) {
		t.Fatal("downloaded bytes do not match uploaded payload")
	}
	if length != int64(len(testPayload)) {
		t.Fatalf("expected length %d, got %d", len(testPayload), length)
	}

	// 4. Download partial range crossing chunk boundary (bytes=50000 to 80000 -> crosses chunk 0 and chunk 1)
	crossContentRange := &ContentRange{
		Start:     50000,
		End:       80000,
		Total:     int64(len(testPayload)),
		IsPartial: true,
	}
	crossReadCloser, crossLength, crossErr := databaseEngine.Download(ctx, testBucket, testKey, crossContentRange)
	if crossErr != nil {
		t.Fatalf("failed to download cross-chunk range: %v", crossErr)
	}
	defer func() {
		_ = crossReadCloser.Close()
	}()

	crossBytes, err := io.ReadAll(crossReadCloser)
	if err != nil {
		t.Fatalf("failed reading cross range: %v", err)
	}
	expectedCrossSlice := testPayload[50000:80001]
	if !bytes.Equal(crossBytes, expectedCrossSlice) {
		t.Fatal("cross-chunk range bytes do not match expected slice")
	}
	if crossLength != int64(len(expectedCrossSlice)) {
		t.Fatalf("expected cross length %d, got %d", len(expectedCrossSlice), crossLength)
	}

	// 5. Test invalid range returns ErrInvalidRange
	invalidContentRange := &ContentRange{
		Start:     90000,
		End:       50000,
		Total:     int64(len(testPayload)),
		IsPartial: true,
	}
	_, _, invalidRangeErr := databaseEngine.Download(ctx, testBucket, testKey, invalidContentRange)
	if invalidRangeErr == nil || !strings.Contains(invalidRangeErr.Error(), "invalid content range") {
		t.Fatalf("expected ErrInvalidRange, got: %v", invalidRangeErr)
	}

	// 6. Test file size exceeding limit
	smallCapBucket := testBucket
	smallCapBucket.MaxFileSizeBytes = 1000
	_, overLimitErr := databaseEngine.Upload(ctx, smallCapBucket, "oversized.dat", bytes.NewReader(testPayload), int64(len(testPayload)), "application/octet-stream")
	if overLimitErr == nil || !strings.Contains(overLimitErr.Error(), "exceeds maximum allowed file size") {
		t.Fatalf("expected ErrPayloadTooLarge, got: %v", overLimitErr)
	}

	// 7. Delete object and verify chunks cascade deleted
	if deleteErr := databaseEngine.Delete(ctx, testBucket, testKey); deleteErr != nil {
		t.Fatalf("failed to delete object: %v", deleteErr)
	}

	var postDeleteChunkCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM file_storage.chunks WHERE object_id = $1", uploadedObject.ID).Scan(&postDeleteChunkCount)
	if postDeleteChunkCount != 0 {
		t.Fatalf("expected 0 chunks after cascade delete, got %d", postDeleteChunkCount)
	}

	// 8. Delete non-existent object returns ErrObjectNotFound
	if deleteNotFoundErr := databaseEngine.Delete(ctx, testBucket, testKey); deleteNotFoundErr == nil || !strings.Contains(deleteNotFoundErr.Error(), "object not found") {
		t.Fatalf("expected ErrObjectNotFound, got: %v", deleteNotFoundErr)
	}

	// 9. Upload and download 0-byte object
	zeroPayloadKey := "empty.dat"
	emptyObject, uploadEmptyErr := databaseEngine.Upload(ctx, testBucket, zeroPayloadKey, bytes.NewReader([]byte{}), 0, "application/octet-stream")
	if uploadEmptyErr != nil {
		t.Fatalf("failed to upload 0-byte object: %v", uploadEmptyErr)
	}
	if emptyObject.SizeBytes != 0 {
		t.Fatalf("expected empty object size 0, got %d", emptyObject.SizeBytes)
	}
	emptyReadCloser, emptyLength, downloadEmptyErr := databaseEngine.Download(ctx, testBucket, zeroPayloadKey, nil)
	if downloadEmptyErr != nil {
		t.Fatalf("failed to download 0-byte object: %v", downloadEmptyErr)
	}
	_ = emptyReadCloser.Close()
	if emptyLength != 0 {
		t.Fatalf("expected empty length 0, got %d", emptyLength)
	}
	_ = databaseEngine.Delete(ctx, testBucket, zeroPayloadKey)

	// 10. Operations with canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, cancelUploadErr := databaseEngine.Upload(canceledCtx, testBucket, "cancel.dat", bytes.NewReader([]byte("test")), 4, "text/plain")
	if cancelUploadErr == nil {
		t.Fatal("expected error on upload with canceled context")
	}

	cancelDuringReadCtx, cancelDuringReadCancel := context.WithCancel(ctx)
	failingCancelOnReadReader := &cancelOnReadReader{
		reader:       bytes.NewReader([]byte("chunk-data-to-insert")),
		cancelCancel: cancelDuringReadCancel,
	}
	_, chunkExecErr := databaseEngine.Upload(cancelDuringReadCtx, testBucket, "chunk-exec-fail.dat", failingCancelOnReadReader, 20, "text/plain")
	if chunkExecErr == nil {
		t.Fatal("expected error on upload when chunk insertion fails")
	}

	_, cancelHeadErr := databaseEngine.Head(canceledCtx, testBucket, testKey)
	if cancelHeadErr == nil {
		t.Fatal("expected error on head with canceled context")
	}

	cancelDeleteErr := databaseEngine.Delete(canceledCtx, testBucket, testKey)
	if cancelDeleteErr == nil {
		t.Fatal("expected error on delete with canceled context")
	}

	_, _, cancelDownloadErr := databaseEngine.Download(canceledCtx, testBucket, testKey, nil)
	if cancelDownloadErr == nil {
		t.Fatal("expected error on download with canceled context")
	}

	// 11. Upload with failing reader
	failingReader := &errorMockReader{}
	_, failingStreamErr := databaseEngine.Upload(ctx, testBucket, "stream-error.dat", failingReader, 100, "text/plain")
	if failingStreamErr == nil {
		t.Fatal("expected error on upload with failing reader")
	}

	// 12. Early closed pipe on chunk download
	earlyCloseKey := "early-close.dat"
	_, uploadEarlyCloseErr := databaseEngine.Upload(ctx, testBucket, earlyCloseKey, bytes.NewReader(testPayload), int64(len(testPayload)), "application/octet-stream")
	if uploadEarlyCloseErr != nil {
		t.Fatalf("failed to upload for early close: %v", uploadEarlyCloseErr)
	}
	earlyCloseReadCloser, _, earlyCloseDownloadErr := databaseEngine.Download(ctx, testBucket, earlyCloseKey, nil)
	if earlyCloseDownloadErr != nil {
		t.Fatalf("failed to initiate download for early close: %v", earlyCloseDownloadErr)
	}
	var smallChunkBuffer [5]byte
	_, _ = earlyCloseReadCloser.Read(smallChunkBuffer[:])
	_ = earlyCloseReadCloser.Close()
	_ = databaseEngine.Delete(ctx, testBucket, earlyCloseKey)

	// 13. Upload with non-existent bucket ID (foreign key error)
	_, foreignKeyErr := databaseEngine.Upload(ctx, Bucket{ID: uuid.NewV7(), Name: "bad-bucket"}, "key.txt", bytes.NewReader([]byte("test")), 4, "text/plain")
	if foreignKeyErr == nil {
		t.Fatal("expected error on upload with non-existent bucket ID")
	}
}

type errorMockReader struct{}

func (errorMockReader) Read(payloadBuffer []byte) (int, error) {
	return 0, errors.New("simulated read stream failure")
}

type cancelOnReadReader struct {
	reader       io.Reader
	cancelCancel context.CancelFunc
}

func (cr *cancelOnReadReader) Read(buffer []byte) (int, error) {
	bytesRead, readErr := cr.reader.Read(buffer)
	if bytesRead > 0 && cr.cancelCancel != nil {
		cr.cancelCancel()
		cr.cancelCancel = nil
	}
	if readErr != nil {
		return bytesRead, fmt.Errorf("read failed: %w", readErr)
	}
	return bytesRead, nil
}
