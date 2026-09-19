package auth

import (
	"context"
	"layr.sh/auth/password"
	"layr.sh/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthControlPlaneHandlerUnit(t *testing.T) {
	ctx := context.Background()
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(nil, configManager)
	controlPlaneHandler.SetKVStore(nil)
	controlPlaneHandler.SetEventBus(nil)
	controlPlaneHandler.SetServiceAccountManager(nil)
	controlPlaneHandler.SetHasher(password.NewHasher())

	// 1. checkScope when serviceAccountManager is nil (passes)
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	if !controlPlaneHandler.checkScope(listRequest, "auth:user.read") {
		t.Fatal("expected checkScope to return true when serviceAccountManager is nil")
	}

	// 2. checkScope with empty bearer
	serviceAccountManager := core.NewServiceAccountManager(nil)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	if !controlPlaneHandler.checkScope(listRequest, "auth:user.read") {
		t.Fatal("expected checkScope to return true when no secret key in header")
	}

	// 3. checkScope with invalid secret key
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_secret_key")
	if controlPlaneHandler.checkScope(invalidKeyRequest, "auth:user.read") {
		t.Fatal("expected checkScope to return false for invalid secret key")
	}

	// 4. Test fallback path where PathValue is empty and path does not have user_id segment
	controlPlaneHandler.SetServiceAccountManager(nil)
	emptyUserPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/other", nil)
	emptyUserPathResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(emptyUserPathResponseRecorder, emptyUserPathRequest)
	if emptyUserPathResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleGetUser with missing user ID in path, got: %d", emptyUserPathResponseRecorder.Code)
	}

	// 5. Test fallback path where PathValue is empty but path has >= 6 segments with users
	fallbackUserPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/01234567-89ab-cdef-0123-456789abcdef", nil)
	extractedID := controlPlaneHandler.extractUserID(fallbackUserPathRequest)
	if extractedID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Fatalf("expected extracted ID from fallback path, got: %s", extractedID)
	}
}
