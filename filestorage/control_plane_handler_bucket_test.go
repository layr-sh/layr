package filestorage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerBucketUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	privilegedAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageBucketWrite,
		},
	}
	privilegedCtx := core.WithAuthContext(context.Background(), privilegedAuthContext)

	readOnlyAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageBucketRead,
		},
	}
	readOnlyCtx := core.WithAuthContext(context.Background(), readOnlyAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(context.Background(), noScopeAuthContext)

	t.Run("handle list buckets unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBuckets(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}
	})

	t.Run("handle create bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		readOnlyRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"test"}`)))
		readOnlyResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(readOnlyResponseRecorder, readOnlyRequest)
		if readOnlyResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for insufficient scope, got %d", readOnlyResponseRecorder.Code)
		}

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(badJSONResponseRecorder, badJSONRequest)
		if badJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for bad JSON, got %d", badJSONResponseRecorder.Code)
		}

		// Empty name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"   "}`)))
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}

		// Invalid bucket name (too short) -> 400
		invalidNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"ab"}`)))
		invalidNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(invalidNameResponseRecorder, invalidNameRequest)
		if invalidNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for short bucket name, got %d", invalidNameResponseRecorder.Code)
		}

		// Invalid bucket name (invalid characters) -> 400
		invalidCharsNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"Invalid_Bucket!"}`)))
		invalidCharsNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(invalidCharsNameResponseRecorder, invalidCharsNameRequest)
		if invalidCharsNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid characters bucket name, got %d", invalidCharsNameResponseRecorder.Code)
		}

		// Invalid backend -> 400
		badBackendRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"valid-name","backend":"unsupported-backend"}`)))
		badBackendResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(badBackendResponseRecorder, badBackendRequest)
		if badBackendResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsupported backend, got %d", badBackendResponseRecorder.Code)
		}
	})

	t.Run("handle get bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets/test-bucket", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodGet, "/v1/_/file-storage/buckets/", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})

	t.Run("handle update bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPatch, "/v1/_/file-storage/buckets/test-bucket", bytes.NewReader([]byte(`{}`)))
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/", bytes.NewReader([]byte(`{}`)))
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/test-bucket", bytes.NewReader([]byte("{invalid-json")))
		badJSONRequest.SetPathValue("bucket", "test-bucket")
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(badJSONResponseRecorder, badJSONRequest)
		if badJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid JSON, got %d", badJSONResponseRecorder.Code)
		}
	})

	t.Run("handle delete bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/file-storage/buckets/test-bucket", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodDelete, "/v1/_/file-storage/buckets/", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})

	t.Run("handle list bucket objects unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets/test-bucket/objects", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBucketObjects(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodGet, "/v1/_/file-storage/buckets//objects", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBucketObjects(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})
}
