package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestFilestorageFullLifecycleE2E(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)

	if startErr := service.Start(ctx); startErr != nil {
		t.Fatalf("failed to start service: %v", startErr)
	}
	defer func() {
		service.Stop()
	}()

	if service.BaseHandler() == nil || service.ControlPlaneHandler() == nil || service.ConfigManager() == nil {
		t.Fatalf("expected non-nil service handlers")
	}

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// Create privileged service account for E2E flow
	createdServiceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "e2e-service-account",
		Scopes: []string{
			core.ScopeFileStorageBucketRead,
			core.ScopeFileStorageBucketWrite,
			core.ScopeFileStorageObjectRead,
			core.ScopeFileStorageObjectWrite,
			core.ScopeFileStorageConfigRead,
			core.ScopeFileStorageConfigWrite,
		},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}
	serviceAccountSecretKey := createdServiceAccount.SecretKey

	// 1. Control Plane: Read Runtime Configuration
	getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/file-storage/config", nil)
	getConfigRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	getConfigResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getConfigResponseRecorder, getConfigRequest)
	if getConfigResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get config, got %d: %s", getConfigResponseRecorder.Code, getConfigResponseRecorder.Body.String())
	}

	// 2. Control Plane: Create Bucket
	createBucketPayload := `{"name":"e2e-lifecycle-bucket","backend":"database","allowed_mime_types":["application/pdf","text/plain"],"max_file_size_bytes":52428800}`
	createBucketRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(createBucketPayload)))
	createBucketRequest.Header.Set("Content-Type", "application/json")
	createBucketRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	createBucketResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createBucketResponseRecorder, createBucketRequest)
	if createBucketResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on create bucket, got %d: %s", createBucketResponseRecorder.Code, createBucketResponseRecorder.Body.String())
	}

	var createdBucket Bucket
	if decodeErr := json.NewDecoder(createBucketResponseRecorder.Body).Decode(&createdBucket); decodeErr != nil {
		t.Fatalf("failed to decode created bucket: %v", decodeErr)
	}

	// 3. Data Plane: Upload Object via PUT
	pdfContent := []byte("%PDF-1.4 Mock PDF content for file storage end-to-end verification")
	uploadObjectRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/e2e-lifecycle-bucket/reports/annual.pdf", bytes.NewReader(pdfContent))
	uploadObjectRequest.Header.Set("Content-Type", "application/pdf")
	uploadObjectRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	uploadObjectResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(uploadObjectResponseRecorder, uploadObjectRequest)
	if uploadObjectResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on upload object, got %d: %s", uploadObjectResponseRecorder.Code, uploadObjectResponseRecorder.Body.String())
	}

	// 4. Data Plane: Head Object via HEAD
	headObjectRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects/e2e-lifecycle-bucket/reports/annual.pdf", nil)
	headObjectRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	headObjectResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(headObjectResponseRecorder, headObjectRequest)
	if headObjectResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on head object, got %d", headObjectResponseRecorder.Code)
	}

	// 5. Data Plane: Download Object via GET
	downloadObjectRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/e2e-lifecycle-bucket/reports/annual.pdf", nil)
	downloadObjectRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	downloadObjectResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(downloadObjectResponseRecorder, downloadObjectRequest)
	if downloadObjectResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on download object, got %d", downloadObjectResponseRecorder.Code)
	}
	downloadedBytes, _ := io.ReadAll(downloadObjectResponseRecorder.Body)
	if string(downloadedBytes) != string(pdfContent) {
		t.Fatalf("expected %q, got %q", string(pdfContent), string(downloadedBytes))
	}

	// 6. Data Plane: Mint Presigned Read Capability URL
	presignPayload := `{"bucket":"e2e-lifecycle-bucket","key":"reports/annual.pdf","operation":"read","expires_in_seconds":300}`
	presignRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(presignPayload)))
	presignRequest.Header.Set("Content-Type", "application/json")
	presignRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	presignResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(presignResponseRecorder, presignRequest)
	if presignResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presign url, got %d: %s", presignResponseRecorder.Code, presignResponseRecorder.Body.String())
	}

	var presignURLResponse PresignURLResponse
	if decodeErr := json.NewDecoder(presignResponseRecorder.Body).Decode(&presignURLResponse); decodeErr != nil {
		t.Fatalf("failed to decode presign response: %v", decodeErr)
	}

	// 7. Data Plane: Anonymous Download via Presigned URL (no service account headers)
	presignedDownloadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, presignURLResponse.URL, nil)
	presignedDownloadRequest.Header.Set("X-Layr-Client-Publishable-Key", kernel.CryptoKeyManager().DerivePublishableKey())
	presignedDownloadResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(presignedDownloadResponseRecorder, presignedDownloadRequest)
	if presignedDownloadResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on presigned download, got %d: %s", presignedDownloadResponseRecorder.Code, presignedDownloadResponseRecorder.Body.String())
	}

	// 8. Data Plane: Delete Object via DELETE
	deleteObjectRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/file-storage/objects/e2e-lifecycle-bucket/reports/annual.pdf", nil)
	deleteObjectRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	deleteObjectResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteObjectResponseRecorder, deleteObjectRequest)
	if deleteObjectResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete object, got %d", deleteObjectResponseRecorder.Code)
	}

	// 9. Control Plane: Delete Bucket via DELETE
	deleteBucketRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/file-storage/buckets/"+createdBucket.Name, nil)
	deleteBucketRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	deleteBucketResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteBucketResponseRecorder, deleteBucketRequest)
	if deleteBucketResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete bucket, got %d", deleteBucketResponseRecorder.Code)
	}
}
