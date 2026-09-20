package filestorage

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
)

func TestFilestorageBaseHandlerS3Unit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	ctx := context.Background()

	t.Run("s3 error formatting unit", func(t *testing.T) {
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3/test-bucket", nil)
		request.Header.Set("X-Request-ID", "req-12345")
		responseRecorder := httptest.NewRecorder()

		baseHandler.writeS3ErrorResponse(responseRecorder, request, http.StatusForbidden, "AccessDenied", "Access denied to bucket.")
		if responseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 status code, got %d", responseRecorder.Code)
		}

		var s3ErrorResponse S3ErrorResponse
		if decodeErr := xml.NewDecoder(responseRecorder.Body).Decode(&s3ErrorResponse); decodeErr != nil {
			t.Fatalf("failed to decode S3 error XML: %v", decodeErr)
		}

		if s3ErrorResponse.Code != "AccessDenied" || s3ErrorResponse.RequestID != "req-12345" {
			t.Fatalf("unexpected S3 error response: %+v", s3ErrorResponse)
		}
	})

	t.Run("unauthenticated s3 requests", func(t *testing.T) {
		endpoints := []struct {
			name    string
			method  string
			url     string
			handler func(http.ResponseWriter, *http.Request)
		}{
			{"list buckets", http.MethodGet, "/v1/file-storage/s3", baseHandler.handleListS3Buckets},
			{"head bucket", http.MethodHead, "/v1/file-storage/s3/test-bucket", baseHandler.handleHeadS3Bucket},
			{"get bucket", http.MethodGet, "/v1/file-storage/s3/test-bucket", baseHandler.handleListS3Bucket},
			{"delete multiple objects", http.MethodPost, "/v1/file-storage/s3/test-bucket?delete", baseHandler.handleDeleteMultipleS3Objects},
			{"get object", http.MethodGet, "/v1/file-storage/s3/test-bucket/test.txt", baseHandler.handleGetS3Object},
			{"head object", http.MethodHead, "/v1/file-storage/s3/test-bucket/test.txt", baseHandler.handleHeadS3Object},
			{"put object", http.MethodPut, "/v1/file-storage/s3/test-bucket/test.txt", baseHandler.handlePutS3Object},
			{"post object", http.MethodPost, "/v1/file-storage/s3/test-bucket/test.txt?uploads", baseHandler.handlePostS3Object},
			{"delete object", http.MethodDelete, "/v1/file-storage/s3/test-bucket/test.txt", baseHandler.handleDeleteS3Object},
		}

		for _, endpoint := range endpoints {
			t.Run(endpoint.name, func(t *testing.T) {
				unauthenticatedRequest := httptest.NewRequestWithContext(ctx, endpoint.method, endpoint.url, nil)
				unauthenticatedResponseRecorder := httptest.NewRecorder()
				endpoint.handler(unauthenticatedResponseRecorder, unauthenticatedRequest)
				if unauthenticatedResponseRecorder.Code != http.StatusUnauthorized {
					t.Fatalf("expected 401 for unauthenticated request on %s, got %d", endpoint.name, unauthenticatedResponseRecorder.Code)
				}
			})
		}
	})

	t.Run("delete multiple objects missing delete param unit", func(t *testing.T) {
		unsupportedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/s3/test-bucket", nil)
		unsupportedResponseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteMultipleS3Objects(unsupportedResponseRecorder, unsupportedRequest)
		// without ?delete, returns 400 InvalidRequest
		if unsupportedResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsupported operation on bucket, got %d", unsupportedResponseRecorder.Code)
		}
	})

	t.Run("delete multiple objects unauthenticated unit", func(t *testing.T) {
		deleteObjectsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/s3/test-bucket?delete", bytes.NewReader([]byte("not-xml")))
		deleteObjectsResponseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteMultipleS3Objects(deleteObjectsResponseRecorder, deleteObjectsRequest)
		if deleteObjectsResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for unauthenticated delete objects, got %d", deleteObjectsResponseRecorder.Code)
		}
	})

	t.Run("authenticateS3 validator unavailable unit", func(t *testing.T) {
		nilValidatorBaseHandler := &BaseHandler{}
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		responseRecorder := httptest.NewRecorder()
		serviceAccount, authorized := nilValidatorBaseHandler.authenticateS3(responseRecorder, request, core.ScopeFileStorageBucketRead)
		if authorized || serviceAccount != nil || responseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 when sigv4 validator is unavailable")
		}
	})

	t.Run("authenticateS3 invalid algorithm unit", func(t *testing.T) {
		badAlgoRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		badAlgoRequest.Header.Set("Authorization", "INVALID-ALGORITHM Credential=KEY/date/region/s3/aws4_request")
		badAlgoResponseRecorder := httptest.NewRecorder()
		serviceAccount, authorized := baseHandler.authenticateS3(badAlgoResponseRecorder, badAlgoRequest, core.ScopeFileStorageBucketRead)
		if authorized || serviceAccount != nil || badAlgoResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid algorithm, got %d", badAlgoResponseRecorder.Code)
		}
	})

	t.Run("authenticateS3 invalid access key unit", func(t *testing.T) {
		invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		invalidKeyRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=NONEXISTENT/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abcdef")
		invalidKeyResponseRecorder := httptest.NewRecorder()
		serviceAccount, authorized := baseHandler.authenticateS3(invalidKeyResponseRecorder, invalidKeyRequest, core.ScopeFileStorageBucketRead)
		if authorized || serviceAccount != nil || invalidKeyResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-existent access key, got %d", invalidKeyResponseRecorder.Code)
		}
	})

	t.Run("handlePostS3Object missing query parameter unit", func(t *testing.T) {
		missingParamPostRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/s3/test-bucket/file.txt", nil)
		missingParamResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePostS3Object(missingParamResponseRecorder, missingParamPostRequest)
		if missingParamResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing uploads or uploadId, got %d", missingParamResponseRecorder.Code)
		}
	})

	t.Run("authenticateS3 expired timestamp unit", func(t *testing.T) {
		expiredTimestampRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		expiredTimestampRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=KEY/20200101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abcdef")
		expiredTimestampRequest.Header.Set("X-Amz-Date", "20200101T000000Z")
		expiredResponseRecorder := httptest.NewRecorder()
		serviceAccount, authorized := baseHandler.authenticateS3(expiredResponseRecorder, expiredTimestampRequest, core.ScopeFileStorageBucketRead)
		if authorized || serviceAccount != nil || expiredResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for expired timestamp, got %d", expiredResponseRecorder.Code)
		}

		invalidAlgorithmRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		invalidAlgorithmRequest.Header.Set("Authorization", "AWS4-HMAC-SHA512 Credential=test/20260921/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
		invalidAlgorithmResponseRecorder := httptest.NewRecorder()
		_, invalidAlgorithmAuthorized := baseHandler.authenticateS3(invalidAlgorithmResponseRecorder, invalidAlgorithmRequest, core.ScopeFileStorageBucketRead)
		if invalidAlgorithmAuthorized || invalidAlgorithmResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on invalid algorithm, got %d", invalidAlgorithmResponseRecorder.Code)
		}

		invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		invalidKeyRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=nonexistent/20260921/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
		invalidKeyRequest.Header.Set("X-Amz-Date", "20260921T000000Z")
		invalidKeyResponseRecorder := httptest.NewRecorder()
		_, invalidKeyAuthorized := baseHandler.authenticateS3(invalidKeyResponseRecorder, invalidKeyRequest, core.ScopeFileStorageBucketRead)
		if invalidKeyAuthorized || invalidKeyResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on invalid access key, got %d", invalidKeyResponseRecorder.Code)
		}
	})

	t.Run("authenticateS3 auth context matching and missing scope unit", func(t *testing.T) {
		serviceAccountID := uuid.NewV7()
		matchingAuthContext := core.AuthContext{
			ServiceAccountID: serviceAccountID.String(),
			JWT: core.JWTClaims{
				Subject: serviceAccountID.String(),
				Role:    "service_role",
				Scope:   core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageObjectWrite,
			},
		}
		matchingCtx := core.WithAuthContext(ctx, matchingAuthContext)

		matchingRequest := httptest.NewRequestWithContext(matchingCtx, http.MethodGet, "/v1/file-storage/s3", nil)
		matchingResponseRecorder := httptest.NewRecorder()
		serviceAccount, authorized := baseHandler.authenticateS3(matchingResponseRecorder, matchingRequest, core.ScopeFileStorageBucketRead)
		if !authorized || serviceAccount == nil {
			t.Fatal("expected authorized service account")
		}

		missingScopeRequest := httptest.NewRequestWithContext(matchingCtx, http.MethodGet, "/v1/file-storage/s3", nil)
		missingScopeResponseRecorder := httptest.NewRecorder()
		_, missingAuthorized := baseHandler.authenticateS3(missingScopeResponseRecorder, missingScopeRequest, core.ScopeFileStorageConfigWrite)
		if missingAuthorized || missingScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on missing scope, got %d", missingScopeResponseRecorder.Code)
		}
	})

	t.Run("s3 handlers with nil database unit", func(t *testing.T) {
		serviceAccountID := uuid.NewV7()
		serviceAccountAuthContext := core.AuthContext{
			ServiceAccountID: serviceAccountID.String(),
			JWT: core.JWTClaims{
				Subject: serviceAccountID.String(),
				Role:    "service_role",
				Scope:   core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageObjectRead + " " + core.ScopeFileStorageObjectWrite,
			},
		}
		authedCtx := core.WithAuthContext(ctx, serviceAccountAuthContext)

		// 1. handleListS3Buckets with nil DB -> 500
		listBucketsRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/file-storage/s3", nil)
		listBucketsResponseRecorder := httptest.NewRecorder()
		baseHandler.handleListS3Buckets(listBucketsResponseRecorder, listBucketsRequest)
		if listBucketsResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on list s3 buckets with nil db, got %d", listBucketsResponseRecorder.Code)
		}

		// 2. handleListS3Bucket with nil DB -> 404
		listBucketRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/file-storage/s3/any-bucket", nil)
		listBucketRequest.SetPathValue("bucket", "any-bucket")
		listBucketResponseRecorder := httptest.NewRecorder()
		baseHandler.handleListS3Bucket(listBucketResponseRecorder, listBucketRequest)
		if listBucketResponseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on list s3 bucket with nil db, got %d", listBucketResponseRecorder.Code)
		}

		// 3. processDeleteMultipleS3Objects with nil DB (bucket resolution fails) -> 404
		deleteObjectsRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/any-bucket?delete", bytes.NewReader([]byte("not-xml")))
		deleteObjectsRequest.SetPathValue("bucket", "any-bucket")
		deleteObjectsResponseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteMultipleS3Objects(deleteObjectsResponseRecorder, deleteObjectsRequest)
		if deleteObjectsResponseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on delete objects with missing bucket, got %d", deleteObjectsResponseRecorder.Code)
		}

		// 4. processCreateMultipartUpload with nil DB (bucket resolution fails) -> 404
		createMultipartRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/any-bucket/file.txt?uploads", nil)
		createMultipartRequest.SetPathValue("bucket", "any-bucket")
		createMultipartRequest.SetPathValue("key", "file.txt")
		createMultipartResponseRecorder := httptest.NewRecorder()
		baseHandler.processCreateMultipartUpload(createMultipartResponseRecorder, createMultipartRequest)
		if createMultipartResponseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on create multipart with nil db, got %d", createMultipartResponseRecorder.Code)
		}

		// 5. processUploadPart with invalid uploadId -> 400
		badUploadIDPartRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/any-bucket/file.txt?uploadId=bad-id&partNumber=1", nil)
		badUploadIDPartRequest.SetPathValue("bucket", "any-bucket")
		badUploadIDPartRequest.SetPathValue("key", "file.txt")
		badUploadIDPartResponseRecorder := httptest.NewRecorder()
		baseHandler.processUploadPart(badUploadIDPartResponseRecorder, badUploadIDPartRequest)
		if badUploadIDPartResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on upload part bad uploadId, got %d", badUploadIDPartResponseRecorder.Code)
		}

		// 6. processUploadPart with bad partNumber -> 400
		badPartNumberRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/any-bucket/file.txt?uploadId="+uuid.NewV7().String()+"&partNumber=0", nil)
		badPartNumberRequest.SetPathValue("bucket", "any-bucket")
		badPartNumberRequest.SetPathValue("key", "file.txt")
		badPartNumberResponseRecorder := httptest.NewRecorder()
		baseHandler.processUploadPart(badPartNumberResponseRecorder, badPartNumberRequest)
		if badPartNumberResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on upload part bad partNumber, got %d", badPartNumberResponseRecorder.Code)
		}

		// 7. processCompleteMultipartUpload with bad uploadId -> 400
		badCompleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/any-bucket/file.txt?uploadId=bad-id", nil)
		badCompleteRequest.SetPathValue("bucket", "any-bucket")
		badCompleteRequest.SetPathValue("key", "file.txt")
		badCompleteResponseRecorder := httptest.NewRecorder()
		baseHandler.processCompleteMultipartUpload(badCompleteResponseRecorder, badCompleteRequest)
		if badCompleteResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on complete multipart bad uploadId, got %d", badCompleteResponseRecorder.Code)
		}

		// 8. processAbortMultipartUpload with bad uploadId -> 400
		badAbortRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/file-storage/s3/any-bucket/file.txt?uploadId=bad-id", nil)
		badAbortRequest.SetPathValue("bucket", "any-bucket")
		badAbortRequest.SetPathValue("key", "file.txt")
		badAbortResponseRecorder := httptest.NewRecorder()
		baseHandler.processAbortMultipartUpload(badAbortResponseRecorder, badAbortRequest)
		if badAbortResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on abort multipart bad uploadId, got %d", badAbortResponseRecorder.Code)
		}
	})

	t.Run("multipart unauthenticated calls unit", func(t *testing.T) {
		unauthenticatedRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/s3/bucket/key", nil)
		uploadPartResponseRecorder := httptest.NewRecorder()
		baseHandler.processUploadPart(uploadPartResponseRecorder, unauthenticatedRequest)
		if uploadPartResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on unauth upload part, got %d", uploadPartResponseRecorder.Code)
		}

		completeMultipartResponseRecorder := httptest.NewRecorder()
		baseHandler.processCompleteMultipartUpload(completeMultipartResponseRecorder, unauthenticatedRequest)
		if completeMultipartResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on unauth complete multipart, got %d", completeMultipartResponseRecorder.Code)
		}

		abortMultipartResponseRecorder := httptest.NewRecorder()
		baseHandler.processAbortMultipartUpload(abortMultipartResponseRecorder, unauthenticatedRequest)
		if abortMultipartResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on unauth abort multipart, got %d", abortMultipartResponseRecorder.Code)
		}
	})

	t.Run("authenticateS3 database pool unavailable", func(t *testing.T) {
		nowTimestamp := time.Now().UTC().Format("20060102T150405Z")
		databaseUnavailableRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		databaseUnavailableRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=anykey/"+nowTimestamp[:8]+"/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abcdef")
		databaseUnavailableRequest.Header.Set("X-Amz-Date", nowTimestamp)
		databaseUnavailableResponseRecorder := httptest.NewRecorder()
		_, isAuthorized := baseHandler.authenticateS3(databaseUnavailableResponseRecorder, databaseUnavailableRequest, core.ScopeFileStorageBucketRead)
		if isAuthorized || databaseUnavailableResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on database pool unavailable during s3 auth, got %d", databaseUnavailableResponseRecorder.Code)
		}
		var s3ErrorResponse S3ErrorResponse
		_ = xml.NewDecoder(databaseUnavailableResponseRecorder.Body).Decode(&s3ErrorResponse)
		if s3ErrorResponse.Message != "Service temporarily unavailable" {
			t.Fatalf("expected Service temporarily unavailable message, got %q", s3ErrorResponse.Message)
		}
	})

	t.Run("authenticateS3 disabled filestorage returns 403", func(t *testing.T) {
		disabledConfigManager := NewConfigManager(nil)
		disabledConfigManager.SetMemoryConfig(Config{Enabled: false})
		disabledBaseHandler := NewBaseHandler(nil, disabledConfigManager, cryptoKeyManager)

		disabledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
		disabledResponseRecorder := httptest.NewRecorder()
		_, isAuth := disabledBaseHandler.authenticateS3(disabledResponseRecorder, disabledRequest, core.ScopeFileStorageBucketRead)
		if isAuth || disabledResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 when filestorage disabled, got %d", disabledResponseRecorder.Code)
		}
	})
}
