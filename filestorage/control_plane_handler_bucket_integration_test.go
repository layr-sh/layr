package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerBucketIntegration(t *testing.T) {
	db, cleanup := setupTestFileStorageDatabase(t)
	defer cleanup()

	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(db)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager, cryptoKeyManager)
	serviceAccountManager := core.NewServiceAccountManager(db)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)

	privilegedAuthContext := core.AuthContext{
		ServiceAccountID: "sa-integration",
		JWT: core.JWTClaims{
			Subject:  "sa-integration",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageBucketWrite,
		},
	}
	privilegedCtx := core.WithAuthContext(context.Background(), privilegedAuthContext)

	// 1. Create Bucket
	createInputJSON := `{"name":"my-test-bucket","backend":"database","allowed_mime_types":["image/png","text/plain"],"max_file_size_bytes":10485760}`
	createRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(createInputJSON)))
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(createResponseRecorder, createRequest)
	if createResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", createResponseRecorder.Code, createResponseRecorder.Body.String())
	}

	var createdBucket Bucket
	if decodeErr := json.NewDecoder(createResponseRecorder.Body).Decode(&createdBucket); decodeErr != nil {
		t.Fatalf("failed to decode created bucket: %v", decodeErr)
	}
	if createdBucket.Name != "my-test-bucket" || createdBucket.Backend != "database" {
		t.Fatalf("unexpected created bucket: %+v", createdBucket)
	}

	// 2. List Buckets
	listRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets", nil)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBuckets(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", listResponseRecorder.Code)
	}

	var listBucketsResponse ListBucketsResponse
	if decodeErr := json.NewDecoder(listResponseRecorder.Body).Decode(&listBucketsResponse); decodeErr != nil {
		t.Fatalf("failed to decode list buckets response: %v", decodeErr)
	}
	if listBucketsResponse.Count < 1 {
		t.Fatalf("expected at least 1 bucket, got %d", listBucketsResponse.Count)
	}

	// 3. Get Bucket
	getRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+createdBucket.Name, nil)
	getRequest.SetPathValue("bucket", createdBucket.Name)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetBucket(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get bucket, got %d", getResponseRecorder.Code)
	}

	var fetchedBucket Bucket
	if decodeErr := json.NewDecoder(getResponseRecorder.Body).Decode(&fetchedBucket); decodeErr != nil {
		t.Fatalf("failed to decode fetched bucket: %v", decodeErr)
	}
	if fetchedBucket.ID != createdBucket.ID || fetchedBucket.Name != "my-test-bucket" {
		t.Fatalf("unexpected fetched bucket: %+v", fetchedBucket)
	}

	// 4. Update Bucket
	updateInputJSON := `{"allowed_mime_types":["application/json"]}`
	updateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/"+createdBucket.Name, bytes.NewReader([]byte(updateInputJSON)))
	updateRequest.SetPathValue("bucket", createdBucket.Name)
	updateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(updateResponseRecorder, updateRequest)
	if updateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update bucket, got %d: %s", updateResponseRecorder.Code, updateResponseRecorder.Body.String())
	}

	var updatedBucket Bucket
	if decodeErr := json.NewDecoder(updateResponseRecorder.Body).Decode(&updatedBucket); decodeErr != nil {
		t.Fatalf("failed to decode updated bucket: %v", decodeErr)
	}
	if len(updatedBucket.AllowedMIMETypes) != 1 || updatedBucket.AllowedMIMETypes[0] != "application/json" {
		t.Fatalf("expected updated allowed_mime_types, got: %v", updatedBucket.AllowedMIMETypes)
	}

	// 5. List Bucket Objects (empty)
	listObjectsRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+updatedBucket.Name+"/objects", nil)
	listObjectsRequest.SetPathValue("bucket", updatedBucket.Name)
	listObjectsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBucketObjects(listObjectsResponseRecorder, listObjectsRequest)
	if listObjectsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list objects, got %d", listObjectsResponseRecorder.Code)
	}

	var listBucketObjectsResponse ListBucketObjectsResponse
	if decodeErr := json.NewDecoder(listObjectsResponseRecorder.Body).Decode(&listBucketObjectsResponse); decodeErr != nil {
		t.Fatalf("failed to decode list objects response: %v", decodeErr)
	}
	if listBucketObjectsResponse.Count != 0 {
		t.Fatalf("expected 0 objects, got %d", listBucketObjectsResponse.Count)
	}

	// 6. Create Bucket with S3 Backend and verify secret masking
	createS3InputJSON := `{"name":"s3-test-bucket","backend":"s3","backend_config":{"endpoint":"http://localhost:4566","bucket":"test","region":"us-east-1","access_key_id":"test-key","secret_access_key":"super-secret-key"}}`
	createS3Request := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(createS3InputJSON)))
	createS3ResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(createS3ResponseRecorder, createS3Request)
	if createS3ResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for s3 bucket, got %d: %s", createS3ResponseRecorder.Code, createS3ResponseRecorder.Body.String())
	}

	var createdS3Bucket Bucket
	if decodeErr := json.NewDecoder(createS3ResponseRecorder.Body).Decode(&createdS3Bucket); decodeErr != nil {
		t.Fatalf("failed to decode created s3 bucket: %v", decodeErr)
	}
	if createdS3Bucket.BackendConfig["secret_access_key"] != "********" {
		t.Fatalf("expected secret_access_key to be masked, got: %v", createdS3Bucket.BackendConfig["secret_access_key"])
	}

	// Get S3 Bucket (verifies masking on handleGetBucket)
	getS3Request := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+createdS3Bucket.Name, nil)
	getS3Request.SetPathValue("bucket", createdS3Bucket.Name)
	getS3ResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetBucket(getS3ResponseRecorder, getS3Request)
	if getS3ResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get s3 bucket, got %d", getS3ResponseRecorder.Code)
	}
	var fetchedS3Bucket Bucket
	_ = json.NewDecoder(getS3ResponseRecorder.Body).Decode(&fetchedS3Bucket)
	if fetchedS3Bucket.BackendConfig["secret_access_key"] != "********" {
		t.Fatalf("expected masked secret_access_key on get s3 bucket")
	}

	// Duplicate Create -> 409 Conflict
	duplicateCreateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(createInputJSON)))
	duplicateCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(duplicateCreateResponseRecorder, duplicateCreateRequest)
	if duplicateCreateResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate create, got %d", duplicateCreateResponseRecorder.Code)
	}

	// Get non-existent bucket -> 404 Not Found
	nonExistentGetRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/non-existent-bucket", nil)
	nonExistentGetRequest.SetPathValue("bucket", "non-existent-bucket")
	nonExistentGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetBucket(nonExistentGetResponseRecorder, nonExistentGetRequest)
	if nonExistentGetResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on get non-existent bucket, got %d", nonExistentGetResponseRecorder.Code)
	}

	// Update non-existent bucket -> 404 Not Found
	nonExistentUpdateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/non-existent-bucket", bytes.NewReader([]byte(`{}`)))
	nonExistentUpdateRequest.SetPathValue("bucket", "non-existent-bucket")
	nonExistentUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(nonExistentUpdateResponseRecorder, nonExistentUpdateRequest)
	if nonExistentUpdateResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on update non-existent bucket, got %d", nonExistentUpdateResponseRecorder.Code)
	}

	// Update invalid backend -> 400 Bad Request
	badBackendUpdateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/"+updatedBucket.Name, bytes.NewReader([]byte(`{"backend":"invalid-backend"}`)))
	badBackendUpdateRequest.SetPathValue("bucket", updatedBucket.Name)
	badBackendUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(badBackendUpdateResponseRecorder, badBackendUpdateRequest)
	if badBackendUpdateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid backend update, got %d", badBackendUpdateResponseRecorder.Code)
	}

	// Update with s3 backend, new secret_access_key, is_public, max_file_size_bytes
	detailedUpdateJSON := `{"is_public":true,"backend":"s3","max_file_size_bytes":20971520,"backend_config":{"endpoint":"http://localhost:4566","bucket":"test","region":"us-east-1","access_key_id":"key","secret_access_key":"secret123"}}`
	detailedUpdateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/"+updatedBucket.Name, bytes.NewReader([]byte(detailedUpdateJSON)))
	detailedUpdateRequest.SetPathValue("bucket", updatedBucket.Name)
	detailedUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(detailedUpdateResponseRecorder, detailedUpdateRequest)
	if detailedUpdateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on detailed update, got %d: %s", detailedUpdateResponseRecorder.Code, detailedUpdateResponseRecorder.Body.String())
	}

	// Update with masked secret_access_key (should skip re-encryption)
	maskedUpdateJSON := `{"backend_config":{"secret_access_key":"********"}}`
	maskedUpdateRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/"+updatedBucket.Name, bytes.NewReader([]byte(maskedUpdateJSON)))
	maskedUpdateRequest.SetPathValue("bucket", updatedBucket.Name)
	maskedUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(maskedUpdateResponseRecorder, maskedUpdateRequest)
	if maskedUpdateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on masked update, got %d", maskedUpdateResponseRecorder.Code)
	}

	// Insert an object directly into DB to test populated listing
	sampleObjectID := uuid.NewV7()
	insertObjectSQL := `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata)
		VALUES ($1, $2, 'folder/sample.txt', 'text/plain', 12, 'fake-checksum', '{"author":"test"}'::jsonb);
	`
	_, insertObjectErr := db.Exec(context.Background(), insertObjectSQL, sampleObjectID, updatedBucket.ID)
	if insertObjectErr != nil {
		t.Fatalf("failed to insert test object: %v", insertObjectErr)
	}

	// List objects populated
	populatedListObjectsRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+updatedBucket.Name+"/objects", nil)
	populatedListObjectsRequest.SetPathValue("bucket", updatedBucket.Name)
	populatedListObjectsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBucketObjects(populatedListObjectsResponseRecorder, populatedListObjectsRequest)
	if populatedListObjectsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on populated list objects, got %d", populatedListObjectsResponseRecorder.Code)
	}
	var populatedListBucketObjectsResponse ListBucketObjectsResponse
	if decodeErr := json.NewDecoder(populatedListObjectsResponseRecorder.Body).Decode(&populatedListBucketObjectsResponse); decodeErr != nil {
		t.Fatalf("failed to decode populated list objects response: %v", decodeErr)
	}
	if populatedListBucketObjectsResponse.Count != 1 || populatedListBucketObjectsResponse.Objects[0].ObjectKey != "folder/sample.txt" {
		t.Fatalf("unexpected populated list objects response: %+v", populatedListBucketObjectsResponse)
	}

	// List objects for non-existent bucket -> 404 Not Found
	nonExistentListObjectsRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodGet, "/v1/_/file-storage/buckets/non-existent-bucket/objects", nil)
	nonExistentListObjectsRequest.SetPathValue("bucket", "non-existent-bucket")
	nonExistentListObjectsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBucketObjects(nonExistentListObjectsResponseRecorder, nonExistentListObjectsRequest)
	if nonExistentListObjectsResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on list objects for non-existent bucket, got %d", nonExistentListObjectsResponseRecorder.Code)
	}

	// Delete non-existent bucket -> 404 Not Found
	nonExistentDeleteRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodDelete, "/v1/_/file-storage/buckets/non-existent-bucket", nil)
	nonExistentDeleteRequest.SetPathValue("bucket", "non-existent-bucket")
	nonExistentDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(nonExistentDeleteResponseRecorder, nonExistentDeleteRequest)
	if nonExistentDeleteResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on delete non-existent bucket, got %d", nonExistentDeleteResponseRecorder.Code)
	}

	// Test checkScope with raw X-Service-Account-Key header (without AuthContext)
	createdServiceAccount, createAccountErr := serviceAccountManager.Create(privilegedCtx, core.CreateServiceAccountInput{
		Name:   "cp-test-service-account",
		Scopes: []string{core.ScopeFileStorageBucketRead, core.ScopeFileStorageBucketWrite},
	})
	if createAccountErr != nil {
		t.Fatalf("failed to create cp test service account: %v", createAccountErr)
	}
	serviceAccountSecretKey := createdServiceAccount.SecretKey

	keyAuthedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/file-storage/buckets", nil)
	keyAuthedRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	keyAuthedResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBuckets(keyAuthedResponseRecorder, keyAuthedRequest)
	if keyAuthedResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on key authenticated list buckets, got %d", keyAuthedResponseRecorder.Code)
	}

	// Canceled context on database calls
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledCtx = core.WithAuthContext(canceledCtx, privilegedAuthContext)

	cancelListRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/_/file-storage/buckets", nil)
	cancelListRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBuckets(cancelListResponseRecorder, cancelListRequest)
	if cancelListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled list buckets, got %d", cancelListResponseRecorder.Code)
	}

	cancelCreateRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"cancel-bucket"}`)))
	cancelCreateRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(cancelCreateResponseRecorder, cancelCreateRequest)
	if cancelCreateResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled create bucket, got %d", cancelCreateResponseRecorder.Code)
	}

	cancelGetRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+updatedBucket.Name, nil)
	cancelGetRequest.SetPathValue("bucket", updatedBucket.Name)
	cancelGetRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetBucket(cancelGetResponseRecorder, cancelGetRequest)
	if cancelGetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled get bucket, got %d", cancelGetResponseRecorder.Code)
	}

	cancelUpdateRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPatch, "/v1/_/file-storage/buckets/"+updatedBucket.Name, bytes.NewReader([]byte(`{}`)))
	cancelUpdateRequest.SetPathValue("bucket", updatedBucket.Name)
	cancelUpdateRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateBucket(cancelUpdateResponseRecorder, cancelUpdateRequest)
	if cancelUpdateResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled update bucket, got %d", cancelUpdateResponseRecorder.Code)
	}

	cancelDeleteRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodDelete, "/v1/_/file-storage/buckets/"+updatedBucket.Name, nil)
	cancelDeleteRequest.SetPathValue("bucket", updatedBucket.Name)
	cancelDeleteRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(cancelDeleteResponseRecorder, cancelDeleteRequest)
	if cancelDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled delete bucket, got %d", cancelDeleteResponseRecorder.Code)
	}

	cancelListObjectsRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/_/file-storage/buckets/"+updatedBucket.Name+"/objects", nil)
	cancelListObjectsRequest.SetPathValue("bucket", updatedBucket.Name)
	cancelListObjectsRequest.Header.Set("X-Service-Account-Key", serviceAccountSecretKey)
	cancelListObjectsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListBucketObjects(cancelListObjectsResponseRecorder, cancelListObjectsRequest)
	if cancelListObjectsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled list bucket objects, got %d", cancelListObjectsResponseRecorder.Code)
	}

	// 7. Delete Buckets
	deleteRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodDelete, "/v1/_/file-storage/buckets/"+updatedBucket.Name, nil)
	deleteRequest.SetPathValue("bucket", updatedBucket.Name)
	deleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete, got %d", deleteResponseRecorder.Code)
	}

	deleteS3Request := httptest.NewRequestWithContext(privilegedCtx, http.MethodDelete, "/v1/_/file-storage/buckets/"+createdS3Bucket.Name, nil)
	deleteS3Request.SetPathValue("bucket", createdS3Bucket.Name)
	deleteS3ResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(deleteS3ResponseRecorder, deleteS3Request)
	if deleteS3ResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete s3 bucket, got %d", deleteS3ResponseRecorder.Code)
	}
}
