package filestorage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"layr.sh/core"
)

func TestFilestorageBaseHandlerPresignUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := service.BaseHandler()
	ctx := context.Background()

	t.Run("invalid json body", func(t *testing.T) {
		invalidJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte("{invalid-json")))
		invalidJSONResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePresignURL(invalidJSONResponseRecorder, invalidJSONRequest)
		if invalidJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid json, got %d", invalidJSONResponseRecorder.Code)
		}
	})

	t.Run("missing bucket or key", func(t *testing.T) {
		missingBucketRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(`{"bucket":"","key":"test.txt","operation":"read"}`)))
		missingBucketResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePresignURL(missingBucketResponseRecorder, missingBucketRequest)
		if missingBucketResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing bucket, got %d", missingBucketResponseRecorder.Code)
		}

		missingKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(`{"bucket":"test-bucket","key":"","operation":"read"}`)))
		missingKeyResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePresignURL(missingKeyResponseRecorder, missingKeyRequest)
		if missingKeyResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing key, got %d", missingKeyResponseRecorder.Code)
		}
	})

	t.Run("invalid operation", func(t *testing.T) {
		badOperationRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(`{"bucket":"test-bucket","key":"test.txt","operation":"invalid_op"}`)))
		badOperationResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePresignURL(badOperationResponseRecorder, badOperationRequest)
		if badOperationResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for bad operation, got %d", badOperationResponseRecorder.Code)
		}
	})

	t.Run("unauthorized caller", func(t *testing.T) {
		unauthorizedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/file-storage/presign", bytes.NewReader([]byte(`{"bucket":"test-bucket","key":"test.txt","operation":"read"}`)))
		unauthorizedResponseRecorder := httptest.NewRecorder()
		baseHandler.handlePresignURL(unauthorizedResponseRecorder, unauthorizedRequest)
		if unauthorizedResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for unauthorized caller, got %d", unauthorizedResponseRecorder.Code)
		}
	})

	t.Run("verify presigned token unit", func(t *testing.T) {
		signingKey := baseHandler.presignSecretKey()
		expiresUnix := time.Now().Add(10 * time.Minute).Unix()
		token := baseHandler.computePresignSignature(signingKey, "photos", "avatar.png", "read", expiresUnix)
		expiresString := strconv.FormatInt(expiresUnix, 10)

		// 1. Valid token
		if !baseHandler.verifyPresignedToken("photos", "avatar.png", "read", token, expiresString) {
			t.Fatalf("expected valid presigned token to verify successfully")
		}

		// 2. Empty token or expires
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "read", "", expiresString) {
			t.Fatalf("expected empty token to fail")
		}
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "read", token, "") {
			t.Fatalf("expected empty expires to fail")
		}

		// 3. Invalid expires format
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "read", token, "invalid-timestamp") {
			t.Fatalf("expected invalid expires timestamp to fail")
		}

		// 4. Expired token
		expiredUnix := time.Now().Add(-10 * time.Minute).Unix()
		expiredToken := baseHandler.computePresignSignature(signingKey, "photos", "avatar.png", "read", expiredUnix)
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "read", expiredToken, strconv.FormatInt(expiredUnix, 10)) {
			t.Fatalf("expected expired presigned token to fail")
		}

		// 5. Tampered token
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "read", token+"tampered", expiresString) {
			t.Fatalf("expected tampered token to fail")
		}

		// 6. Mismatched bucket or key or operation
		if baseHandler.verifyPresignedToken("other-bucket", "avatar.png", "read", token, expiresString) {
			t.Fatalf("expected mismatched bucket to fail")
		}
		if baseHandler.verifyPresignedToken("photos", "other-avatar.png", "read", token, expiresString) {
			t.Fatalf("expected mismatched key to fail")
		}
		if baseHandler.verifyPresignedToken("photos", "avatar.png", "write", token, expiresString) {
			t.Fatalf("expected mismatched operation to fail")
		}
	})

	t.Run("disabled filestorage configuration returns 403 on presign", func(t *testing.T) {
		disabledService := NewService(kernel)
		disabledService.ConfigManager().SetMemoryConfig(Config{Enabled: false})
		disabledBaseHandler := disabledService.BaseHandler()

		disabledRequest := httptest.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"/v1/file-storage/presign",
			bytes.NewReader([]byte(`{"bucket":"test-bucket","key":"test.txt","operation":"read"}`)),
		)
		disabledResponseRecorder := httptest.NewRecorder()
		disabledBaseHandler.handlePresignURL(disabledResponseRecorder, disabledRequest)
		if disabledResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on presign when disabled, got %d", disabledResponseRecorder.Code)
		}
	})
}
