package filestorage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestFilestorageBaseHandlerRESTUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := service.BaseHandler()
	ctx := context.Background()

	t.Run("download object parameter validation", func(t *testing.T) {
		// Missing bucket and key
		missingParametersRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects//", nil)
		missingParametersResponseRecorder := httptest.NewRecorder()
		baseHandler.handleDownloadObject(missingParametersResponseRecorder, missingParametersRequest)
		if missingParametersResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing parameters, got %d", missingParametersResponseRecorder.Code)
		}
	})

	t.Run("head object parameter validation", func(t *testing.T) {
		// Missing bucket and key
		missingParametersRequest := httptest.NewRequestWithContext(ctx, http.MethodHead, "/v1/file-storage/objects//", nil)
		missingParametersResponseRecorder := httptest.NewRecorder()
		baseHandler.handleHeadObject(missingParametersResponseRecorder, missingParametersRequest)
		if missingParametersResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing parameters, got %d", missingParametersResponseRecorder.Code)
		}
	})

	t.Run("upload object parameter validation", func(t *testing.T) {
		// Missing bucket and key
		missingParametersRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects//", bytes.NewReader([]byte("data")))
		missingParametersResponseRecorder := httptest.NewRecorder()
		baseHandler.handleUploadObject(missingParametersResponseRecorder, missingParametersRequest)
		if missingParametersResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing parameters, got %d", missingParametersResponseRecorder.Code)
		}
	})

	t.Run("delete object parameter validation", func(t *testing.T) {
		// Missing bucket and key
		missingParametersRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/file-storage/objects//", nil)
		missingParametersResponseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteObject(missingParametersResponseRecorder, missingParametersRequest)
		if missingParametersResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing parameters, got %d", missingParametersResponseRecorder.Code)
		}
	})

	t.Run("range header parser unit", func(t *testing.T) {
		// Invalid prefix
		invalidPrefixContentRange, invalidPrefixErr := parseRangeHeader("chars=0-10")
		if invalidPrefixErr == nil || invalidPrefixContentRange != nil {
			t.Fatalf("expected error on invalid range prefix")
		}

		// Invalid parts
		invalidPartsContentRange, invalidPartsErr := parseRangeHeader("bytes=0-10-20")
		if invalidPartsErr == nil || invalidPartsContentRange != nil {
			t.Fatalf("expected error on invalid parts")
		}

		// Non-integer start
		badStartContentRange, badStartErr := parseRangeHeader("bytes=abc-10")
		if badStartErr == nil || badStartContentRange != nil {
			t.Fatalf("expected error on non-integer start")
		}

		// Non-integer end
		badEndContentRange, badEndErr := parseRangeHeader("bytes=10-xyz")
		if badEndErr == nil || badEndContentRange != nil {
			t.Fatalf("expected error on non-integer end")
		}

		// Start greater than end
		invertedRangeContentRange, invertedRangeErr := parseRangeHeader("bytes=50-10")
		if invertedRangeErr == nil || invertedRangeContentRange != nil {
			t.Fatalf("expected error on inverted range")
		}

		// Valid range
		validContentRange, validErr := parseRangeHeader("bytes=0-499")
		if validErr != nil || validContentRange == nil {
			t.Fatalf("expected valid range, got error: %v", validErr)
		}
		if validContentRange.Start != 0 || validContentRange.End != 499 || !validContentRange.IsPartial {
			t.Fatalf("unexpected content range values: %+v", validContentRange)
		}
	})

	t.Run("authorization unit check", func(t *testing.T) {
		bucket := &Bucket{
			ID:       uuid.NewV7(),
			Name:     "private-bucket",
			IsPublic: false,
		}

		unauthorizedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/private-bucket/image.png", nil)
		if baseHandler.authorizeRESTRequest(unauthorizedRequest, bucket, "image.png", core.ScopeFileStorageObjectRead) {
			t.Fatalf("expected unauthorized request to fail on private bucket")
		}

		publicBucket := &Bucket{
			ID:       uuid.NewV7(),
			Name:     "public-bucket",
			IsPublic: true,
		}
		if !baseHandler.authorizeRESTRequest(unauthorizedRequest, publicBucket, "image.png", core.ScopeFileStorageObjectRead) {
			t.Fatalf("expected public bucket read to succeed for anonymous request")
		}
		if baseHandler.authorizeRESTRequest(unauthorizedRequest, publicBucket, "image.png", core.ScopeFileStorageObjectWrite) {
			t.Fatalf("expected public bucket write to fail for anonymous request")
		}

		// With authenticated user context
		userAuthContext := core.AuthContext{
			UserID: "user-1",
			JWT: core.JWTClaims{
				Subject: "user-1",
				Role:    "authenticated",
			},
		}
		authenticatedRequest := httptest.NewRequestWithContext(
			core.WithAuthContext(ctx, userAuthContext),
			http.MethodPut,
			"/v1/file-storage/objects/private-bucket/image.png",
			nil,
		)
		if !baseHandler.authorizeRESTRequest(authenticatedRequest, bucket, "image.png", core.ScopeFileStorageObjectWrite) {
			t.Fatalf("expected authenticated user to be authorized for write")
		}
	})

	t.Run("disabled filestorage configuration returns 403", func(t *testing.T) {
		disabledService := NewService(kernel)
		disabledService.configManager.SetMemoryConfig(Config{Enabled: false})
		disabledBaseHandler := disabledService.BaseHandler()

		disabledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/objects/b/k", nil)
		disabledRequest.SetPathValue("bucket", "b")
		disabledRequest.SetPathValue("key", "k")

		// Download
		downloadResponseRecorder := httptest.NewRecorder()
		disabledBaseHandler.handleDownloadObject(downloadResponseRecorder, disabledRequest)
		if downloadResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on download when disabled, got %d", downloadResponseRecorder.Code)
		}

		// Head
		headResponseRecorder := httptest.NewRecorder()
		disabledBaseHandler.handleHeadObject(headResponseRecorder, disabledRequest)
		if headResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on head when disabled, got %d", headResponseRecorder.Code)
		}

		// Upload
		uploadResponseRecorder := httptest.NewRecorder()
		disabledBaseHandler.handleUploadObject(uploadResponseRecorder, disabledRequest)
		if uploadResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on upload when disabled, got %d", uploadResponseRecorder.Code)
		}

		// Delete
		deleteResponseRecorder := httptest.NewRecorder()
		disabledBaseHandler.handleDeleteObject(deleteResponseRecorder, disabledRequest)
		if deleteResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on delete when disabled, got %d", deleteResponseRecorder.Code)
		}
	})
}
