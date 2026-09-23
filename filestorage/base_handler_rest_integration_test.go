package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
)

func TestFilestorageBaseHandlerRESTIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	baseHandler := service.BaseHandler()
	ctx := context.Background()

	// Create service account for testing
	createdServiceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "rest-test-service-account",
		Scopes: []string{
			core.ScopeFileStorageObjectRead,
			core.ScopeFileStorageObjectWrite,
		},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}
	serviceAccountSecretKey := createdServiceAccount.SecretKey

	// Create test bucket in database
	bucketID := uuid.NewV7()
	const insertBucketSQL = `
		INSERT INTO file_storage.buckets (
			id, name, is_public, backend, allowed_mime_types, max_file_size_bytes
		) VALUES ($1, $2, $3, $4, $5, $6);
	`
	_, insertErr := kernel.DB().Exec(ctx, insertBucketSQL, bucketID, "test-rest-bucket", false, "database", []string{"text/plain", "application/octet-stream"}, 10485760)
	if insertErr != nil {
		t.Fatalf("failed to insert test bucket: %v", insertErr)
	}

	payloadData := []byte("Hello, World from REST filestorage integration test!")

	// 1. Upload Object via PUT
	uploadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", bytes.NewReader(payloadData))
	uploadRequest.SetPathValue("bucket", "test-rest-bucket")
	uploadRequest.SetPathValue("key", "notes/hello.txt")
	uploadRequest.Header.Set("Content-Type", "text/plain")
	uploadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	uploadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(uploadResponseRecorder, uploadRequest)
	if uploadResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on upload, got %d: %s", uploadResponseRecorder.Code, uploadResponseRecorder.Body.String())
	}

	var uploadedObject Object
	if decodeErr := json.NewDecoder(uploadResponseRecorder.Body).Decode(&uploadedObject); decodeErr != nil {
		t.Fatalf("failed to decode uploaded object metadata: %v", decodeErr)
	}
	if uploadedObject.ObjectKey != "notes/hello.txt" || uploadedObject.SizeBytes != int64(len(payloadData)) {
		t.Fatalf("unexpected uploaded object: %+v", uploadedObject)
	}

	// 2. Head Object via HEAD
	headRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	headRequest.SetPathValue("bucket", "test-rest-bucket")
	headRequest.SetPathValue("key", "notes/hello.txt")
	headRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	headResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadObject(headResponseRecorder, headRequest)
	if headResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on head object, got %d", headResponseRecorder.Code)
	}
	if headResponseRecorder.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("expected Content-Type text/plain, got %s", headResponseRecorder.Header().Get("Content-Type"))
	}
	if headResponseRecorder.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("expected Accept-Ranges bytes, got %s", headResponseRecorder.Header().Get("Accept-Ranges"))
	}

	// 3. Download Object via GET
	downloadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	downloadRequest.SetPathValue("bucket", "test-rest-bucket")
	downloadRequest.SetPathValue("key", "notes/hello.txt")
	downloadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	downloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(downloadResponseRecorder, downloadRequest)
	if downloadResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on download, got %d", downloadResponseRecorder.Code)
	}
	downloadedBody, _ := io.ReadAll(downloadResponseRecorder.Body)
	if string(downloadedBody) != string(payloadData) {
		t.Fatalf("expected %q, got %q", string(payloadData), string(downloadedBody))
	}

	// 4. Download with Range Header (bytes=0-4 -> "Hello")
	rangeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	rangeRequest.SetPathValue("bucket", "test-rest-bucket")
	rangeRequest.SetPathValue("key", "notes/hello.txt")
	rangeRequest.Header.Set("Range", "bytes=0-4")
	rangeRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	rangeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(rangeResponseRecorder, rangeRequest)
	if rangeResponseRecorder.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content on range request, got %d", rangeResponseRecorder.Code)
	}
	rangeBody, _ := io.ReadAll(rangeResponseRecorder.Body)
	if string(rangeBody) != "Hello" {
		t.Fatalf("expected %q, got %q", "Hello", string(rangeBody))
	}
	if !strings.HasPrefix(rangeResponseRecorder.Header().Get("Content-Range"), "bytes 0-4/") {
		t.Fatalf("unexpected Content-Range: %s", rangeResponseRecorder.Header().Get("Content-Range"))
	}

	// 5. Presign URL for Object Download
	presignInputJSON := `{"bucket":"test-rest-bucket","key":"notes/hello.txt","operation":"read","expires_in_seconds":600}`
	presignRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignInputJSON)))
	presignRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(presignResponseRecorder, presignRequest)
	if presignResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presign url, got %d: %s", presignResponseRecorder.Code, presignResponseRecorder.Body.String())
	}

	var presignURLResponse PresignURLResponse
	if decodeErr := json.NewDecoder(presignResponseRecorder.Body).Decode(&presignURLResponse); decodeErr != nil {
		t.Fatalf("failed to decode presign response: %v", decodeErr)
	}
	if presignURLResponse.URL == "" {
		t.Fatalf("expected non-empty presigned url")
	}

	// 5.1 Presign URL with operation "get" and default expiration
	presignGetInputJSON := `{"bucket":"test-rest-bucket","key":"notes/hello.txt","operation":"get","expires_in_seconds":0}`
	presignGetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignGetInputJSON)))
	presignGetRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(presignGetResponseRecorder, presignGetRequest)
	if presignGetResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presign get url, got %d", presignGetResponseRecorder.Code)
	}

	// 5.2 Presign URL with operation "put" and maximum expiration cap
	presignPutInputJSON := `{"bucket":"test-rest-bucket","key":"notes/put.txt","operation":"put","expires_in_seconds":9999999}`
	presignPutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignPutInputJSON)))
	presignPutRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignPutResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(presignPutResponseRecorder, presignPutRequest)
	if presignPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presign put url, got %d", presignPutResponseRecorder.Code)
	}

	// 5.3 Presign URL on non-existent bucket -> 404
	presignMissingBucketInputJSON := `{"bucket":"non-existent-bucket","key":"notes/file.txt","operation":"read"}`
	presignMissingBucketRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignMissingBucketInputJSON)))
	presignMissingBucketRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignMissingBucketResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(presignMissingBucketResponseRecorder, presignMissingBucketRequest)
	if presignMissingBucketResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on presign non-existent bucket, got %d", presignMissingBucketResponseRecorder.Code)
	}

	// 6. Anonymous Download via Presigned URL
	presignedDownloadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, presignURLResponse.URL, nil)
	presignedDownloadRequest.SetPathValue("bucket", "test-rest-bucket")
	presignedDownloadRequest.SetPathValue("key", "notes/hello.txt")
	presignedDownloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(presignedDownloadResponseRecorder, presignedDownloadRequest)
	if presignedDownloadResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presigned download, got %d: %s", presignedDownloadResponseRecorder.Code, presignedDownloadResponseRecorder.Body.String())
	}
	presignedBody, _ := io.ReadAll(presignedDownloadResponseRecorder.Body)
	if string(presignedBody) != string(payloadData) {
		t.Fatalf("expected %q from presigned download, got %q", string(payloadData), string(presignedBody))
	}

	// 7. Delete Object via DELETE
	deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	deleteRequest.SetPathValue("bucket", "test-rest-bucket")
	deleteRequest.SetPathValue("key", "notes/hello.txt")
	deleteRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteObject(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete, got %d", deleteResponseRecorder.Code)
	}

	// 8. Verify Download Returns 404 After Deletion
	notFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	notFoundRequest.SetPathValue("bucket", "test-rest-bucket")
	notFoundRequest.SetPathValue("key", "notes/hello.txt")
	notFoundRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	notFoundResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(notFoundResponseRecorder, notFoundRequest)
	if notFoundResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found after deletion, got %d", notFoundResponseRecorder.Code)
	}

	// 9. Head Object 404 on non-existent key
	missingHeadRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects/test-rest-bucket/notes/missing.txt", nil)
	missingHeadRequest.SetPathValue("bucket", "test-rest-bucket")
	missingHeadRequest.SetPathValue("key", "notes/missing.txt")
	missingHeadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	missingHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadObject(missingHeadResponseRecorder, missingHeadRequest)
	if missingHeadResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on head non-existent key, got %d", missingHeadResponseRecorder.Code)
	}

	// 10. Bucket Not Found (404) on all REST endpoints
	endpoints := []struct {
		name    string
		method  string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"download", http.MethodGet, baseHandler.handleDownloadObject},
		{"head", http.MethodHead, baseHandler.handleHeadObject},
		{"upload", http.MethodPut, baseHandler.handleUploadObject},
		{"delete", http.MethodDelete, baseHandler.handleDeleteObject},
	}
	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("NotFound_%s", endpoint.name), func(t *testing.T) {
			nonExistentRequest := httptest.NewRequestWithContext(ctx, endpoint.method, "/v1/file-storage/objects/non-existent-bucket/file.txt", bytes.NewReader([]byte("data")))
			nonExistentRequest.SetPathValue("bucket", "non-existent-bucket")
			nonExistentRequest.SetPathValue("key", "file.txt")
			nonExistentRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
			nonExistentResponseRecorder := httptest.NewRecorder()
			endpoint.handler(nonExistentResponseRecorder, nonExistentRequest)
			if nonExistentResponseRecorder.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for %s on non-existent bucket, got %d", endpoint.name, nonExistentResponseRecorder.Code)
			}
		})
	}

	// 11. Access Denied (403) on private bucket without authentication
	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("Forbidden_%s", endpoint.name), func(t *testing.T) {
			unauthorizedRequest := httptest.NewRequestWithContext(ctx, endpoint.method, "/v1/file-storage/objects/test-rest-bucket/file.txt", bytes.NewReader([]byte("data")))
			unauthorizedRequest.SetPathValue("bucket", "test-rest-bucket")
			unauthorizedRequest.SetPathValue("key", "file.txt")
			unauthorizedResponseRecorder := httptest.NewRecorder()
			endpoint.handler(unauthorizedResponseRecorder, unauthorizedRequest)
			if unauthorizedResponseRecorder.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for %s without authentication, got %d", endpoint.name, unauthorizedResponseRecorder.Code)
			}
		})
	}

	// 12. Upload Validations: Disallowed MIME type
	disallowedMIMERequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/photo.jpg", bytes.NewReader([]byte("image-data")))
	disallowedMIMERequest.SetPathValue("bucket", "test-rest-bucket")
	disallowedMIMERequest.SetPathValue("key", "photo.jpg")
	disallowedMIMERequest.Header.Set("Content-Type", "image/jpeg")
	disallowedMIMERequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	disallowedMIMEResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(disallowedMIMEResponseRecorder, disallowedMIMERequest)
	if disallowedMIMEResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for disallowed MIME type, got %d", disallowedMIMEResponseRecorder.Code)
	}

	// 13. Upload Validations: Exceeds MaxFileSizeBytes
	oversizedRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/huge.txt", bytes.NewReader([]byte("huge")))
	oversizedRequest.SetPathValue("bucket", "test-rest-bucket")
	oversizedRequest.SetPathValue("key", "huge.txt")
	oversizedRequest.ContentLength = 104857600 // 100MB, bucket limit is 10MB
	oversizedRequest.Header.Set("Content-Type", "text/plain")
	oversizedRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	oversizedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(oversizedResponseRecorder, oversizedRequest)
	if oversizedResponseRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized file upload, got %d", oversizedResponseRecorder.Code)
	}

	// 14. Upload with Default Content-Type (empty header)
	defaultContentTypeRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/default.bin", bytes.NewReader([]byte("binary-data")))
	defaultContentTypeRequest.SetPathValue("bucket", "test-rest-bucket")
	defaultContentTypeRequest.SetPathValue("key", "default.bin")
	defaultContentTypeRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	defaultContentTypeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(defaultContentTypeResponseRecorder, defaultContentTypeRequest)
	if defaultContentTypeResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for default content-type upload, got %d: %s", defaultContentTypeResponseRecorder.Code, defaultContentTypeResponseRecorder.Body.String())
	}
	_ = baseHandler.databaseEngine.Delete(ctx, Bucket{ID: bucketID, Name: "test-rest-bucket"}, "default.bin")

	// 15. Presigned URL for Write Operation
	presignWriteInputJSON := `{"bucket":"test-rest-bucket","key":"presigned-upload.txt","operation":"write","expires_in_seconds":600}`
	presignWriteRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignWriteInputJSON)))
	presignWriteRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignWriteResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(presignWriteResponseRecorder, presignWriteRequest)
	if presignWriteResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presign write, got %d: %s", presignWriteResponseRecorder.Code, presignWriteResponseRecorder.Body.String())
	}

	var presignWritePresignURLResponse PresignURLResponse
	_ = json.NewDecoder(presignWriteResponseRecorder.Body).Decode(&presignWritePresignURLResponse)

	presignedUploadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, presignWritePresignURLResponse.URL, bytes.NewReader([]byte("uploaded-via-presigned-url")))
	presignedUploadRequest.SetPathValue("bucket", "test-rest-bucket")
	presignedUploadRequest.SetPathValue("key", "presigned-upload.txt")
	presignedUploadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(presignedUploadResponseRecorder, presignedUploadRequest)
	if presignedUploadResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on presigned write upload, got %d: %s", presignedUploadResponseRecorder.Code, presignedUploadResponseRecorder.Body.String())
	}
	_ = baseHandler.databaseEngine.Delete(ctx, Bucket{ID: bucketID, Name: "test-rest-bucket"}, "presigned-upload.txt")

	// 16. Engine Resolution Error (when backend is unsupported) -> 500
	savedDatabaseEngine := baseHandler.databaseEngine
	_, _ = kernel.DB().Exec(ctx, "UPDATE file_storage.buckets SET backend = 'unsupported' WHERE name = 'test-rest-bucket'")

	nilEngineDownloadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	nilEngineDownloadRequest.SetPathValue("bucket", "test-rest-bucket")
	nilEngineDownloadRequest.SetPathValue("key", "notes/hello.txt")
	nilEngineDownloadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	nilEngineDownloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(nilEngineDownloadResponseRecorder, nilEngineDownloadRequest)
	if nilEngineDownloadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unsupported backend download, got %d", nilEngineDownloadResponseRecorder.Code)
	}

	nilEngineHeadRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	nilEngineHeadRequest.SetPathValue("bucket", "test-rest-bucket")
	nilEngineHeadRequest.SetPathValue("key", "notes/hello.txt")
	nilEngineHeadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	nilEngineHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadObject(nilEngineHeadResponseRecorder, nilEngineHeadRequest)
	if nilEngineHeadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unsupported backend head, got %d", nilEngineHeadResponseRecorder.Code)
	}

	nilEngineUploadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", bytes.NewReader([]byte("data")))
	nilEngineUploadRequest.SetPathValue("bucket", "test-rest-bucket")
	nilEngineUploadRequest.SetPathValue("key", "notes/hello.txt")
	nilEngineUploadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	nilEngineUploadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(nilEngineUploadResponseRecorder, nilEngineUploadRequest)
	if nilEngineUploadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unsupported backend upload, got %d", nilEngineUploadResponseRecorder.Code)
	}

	nilEngineDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	nilEngineDeleteRequest.SetPathValue("bucket", "test-rest-bucket")
	nilEngineDeleteRequest.SetPathValue("key", "notes/hello.txt")
	nilEngineDeleteRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	nilEngineDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteObject(nilEngineDeleteResponseRecorder, nilEngineDeleteRequest)
	if nilEngineDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unsupported backend delete, got %d", nilEngineDeleteResponseRecorder.Code)
	}

	_, _ = kernel.DB().Exec(ctx, "UPDATE file_storage.buckets SET backend = 'database' WHERE name = 'test-rest-bucket'")

	// 17. Engine Operation Error (when engine returns non-404 error) -> 500
	baseHandler.databaseEngine = NewEngine(&mockFailingDriver{})

	failingDownloadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	failingDownloadRequest.SetPathValue("bucket", "test-rest-bucket")
	failingDownloadRequest.SetPathValue("key", "notes/hello.txt")
	failingDownloadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	failingDownloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(failingDownloadResponseRecorder, failingDownloadRequest)
	if failingDownloadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing download, got %d", failingDownloadResponseRecorder.Code)
	}

	failingHeadRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	failingHeadRequest.SetPathValue("bucket", "test-rest-bucket")
	failingHeadRequest.SetPathValue("key", "notes/hello.txt")
	failingHeadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	failingHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadObject(failingHeadResponseRecorder, failingHeadRequest)
	if failingHeadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing head, got %d", failingHeadResponseRecorder.Code)
	}

	failingUploadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", bytes.NewReader([]byte("data")))
	failingUploadRequest.SetPathValue("bucket", "test-rest-bucket")
	failingUploadRequest.SetPathValue("key", "notes/hello.txt")
	failingUploadRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	failingUploadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(failingUploadResponseRecorder, failingUploadRequest)
	if failingUploadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing upload, got %d", failingUploadResponseRecorder.Code)
	}

	failingDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	failingDeleteRequest.SetPathValue("bucket", "test-rest-bucket")
	failingDeleteRequest.SetPathValue("key", "notes/hello.txt")
	failingDeleteRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	failingDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteObject(failingDeleteResponseRecorder, failingDeleteRequest)
	if failingDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing delete, got %d", failingDeleteResponseRecorder.Code)
	}

	baseHandler.databaseEngine = savedDatabaseEngine

	// 18. Canceled context on resolveBucket
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, cancelQueryBucketErr := baseHandler.resolveBucket(canceledCtx, "test-rest-bucket")
	if cancelQueryBucketErr == nil {
		t.Fatal("expected error on resolveBucket with canceled context")
	}

	// Canceled context error on REST handlers (database pool query error -> 500)
	canceledDownloadRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	canceledDownloadRequest.SetPathValue("bucket", "test-rest-bucket")
	canceledDownloadRequest.SetPathValue("key", "notes/hello.txt")
	canceledDownloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDownloadObject(canceledDownloadResponseRecorder, canceledDownloadRequest)
	if canceledDownloadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled download, got %d", canceledDownloadResponseRecorder.Code)
	}

	canceledHeadRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodHead, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	canceledHeadRequest.SetPathValue("bucket", "test-rest-bucket")
	canceledHeadRequest.SetPathValue("key", "notes/hello.txt")
	canceledHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadObject(canceledHeadResponseRecorder, canceledHeadRequest)
	if canceledHeadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled head, got %d", canceledHeadResponseRecorder.Code)
	}

	canceledUploadRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPut, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", bytes.NewReader([]byte("test")))
	canceledUploadRequest.SetPathValue("bucket", "test-rest-bucket")
	canceledUploadRequest.SetPathValue("key", "notes/hello.txt")
	uploadErrResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUploadObject(uploadErrResponseRecorder, canceledUploadRequest)
	if uploadErrResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled upload, got %d", uploadErrResponseRecorder.Code)
	}

	canceledDeleteRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodDelete, "/v1/file-storage/objects/test-rest-bucket/notes/hello.txt", nil)
	canceledDeleteRequest.SetPathValue("bucket", "test-rest-bucket")
	canceledDeleteRequest.SetPathValue("key", "notes/hello.txt")
	deleteErrResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteObject(deleteErrResponseRecorder, canceledDeleteRequest)
	if deleteErrResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled delete, got %d", deleteErrResponseRecorder.Code)
	}

	// Presign with service account auth context
	serviceAccountAuthContext := core.AuthContext{
		ServiceAccountID: createdServiceAccount.ID,
		JWT: core.JWTClaims{
			Subject: createdServiceAccount.ID,
			Role:    "service_role",
			Scope:   core.ScopeFileStorageObjectRead,
		},
	}
	serviceAccountPresignRequest := httptest.NewRequestWithContext(
		core.WithAuthContext(ctx, serviceAccountAuthContext),
		http.MethodPost,
		"/v1/file-storage/presign",
		bytes.NewReader([]byte(`{"bucket":"test-rest-bucket","key":"notes/hello.txt","operation":"read"}`)),
	)
	serviceAccountPresignResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(serviceAccountPresignResponseRecorder, serviceAccountPresignRequest)
	if serviceAccountPresignResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on sa presign, got %d", serviceAccountPresignResponseRecorder.Code)
	}

	// Presign with authenticated user auth context
	userAuthContext := core.AuthContext{
		UserID: "user-123",
		JWT: core.JWTClaims{
			Subject: "user-123",
			Role:    "authenticated",
		},
	}
	userPresignRequest := httptest.NewRequestWithContext(
		core.WithAuthContext(ctx, userAuthContext),
		http.MethodPost,
		"/v1/file-storage/presign",
		bytes.NewReader([]byte(`{"bucket":"test-rest-bucket","key":"notes/hello.txt","operation":"read"}`)),
	)
	userPresignResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(userPresignResponseRecorder, userPresignRequest)
	if userPresignResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on user presign, got %d", userPresignResponseRecorder.Code)
	}

	// Presign with canceled context -> 500
	canceledPresignRequest := httptest.NewRequestWithContext(
		core.WithAuthContext(canceledCtx, userAuthContext),
		http.MethodPost,
		"/v1/file-storage/presign",
		bytes.NewReader([]byte(`{"bucket":"test-rest-bucket","key":"notes/hello.txt","operation":"read"}`)),
	)
	canceledPresignResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePresignURL(canceledPresignResponseRecorder, canceledPresignRequest)
	if canceledPresignResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled presign, got %d", canceledPresignResponseRecorder.Code)
	}

	// 19. Bucket with NULL backend_config and malformed backend_config
	const insertNullConfigSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes)
		VALUES ($1, 'null-config-bucket', true, 'database', NULL, '{}', 1048576);
	`
	nullConfigBucketID := uuid.NewV7()
	_, insertNullErr := kernel.DB().Exec(ctx, insertNullConfigSQL, nullConfigBucketID)
	if insertNullErr == nil {
		nullBucket, _ := baseHandler.resolveBucket(ctx, "null-config-bucket")
		if nullBucket != nil && nullBucket.BackendConfig == nil {
			t.Fatal("expected non-nil BackendConfig")
		}
	}

	const insertBadConfigSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes)
		VALUES ($1, 'bad-config-bucket', true, 'database', 'not-valid-json'::bytea, '{}', 1048576);
	`
	badConfigBucketID := uuid.NewV7()
	_, insertBadErr := kernel.DB().Exec(ctx, insertBadConfigSQL, badConfigBucketID)
	if insertBadErr == nil {
		badBucket, _ := baseHandler.resolveBucket(ctx, "bad-config-bucket")
		if badBucket != nil && badBucket.BackendConfig == nil {
			t.Fatal("expected non-nil BackendConfig")
		}
	}
}

type mockFailingDriver struct{}

func (mockFailingDriver) Upload(_ context.Context, _ Bucket, _ string, reader io.Reader, _ int64, _ string) (*Object, error) {
	if closer, ok := reader.(io.Closer); ok {
		_ = closer.Close()
	}
	time.Sleep(10 * time.Millisecond)
	return nil, fmt.Errorf("simulated upload failure")
}
func (mockFailingDriver) Head(context.Context, Bucket, string) (*Object, error) {
	return nil, fmt.Errorf("simulated head failure")
}
func (mockFailingDriver) Delete(context.Context, Bucket, string) error {
	return fmt.Errorf("simulated delete failure")
}
func (mockFailingDriver) Download(context.Context, Bucket, string, *ContentRange) (io.ReadCloser, int64, error) {
	return nil, 0, fmt.Errorf("simulated download failure")
}
