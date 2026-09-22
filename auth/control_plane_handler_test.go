package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	serviceAccountManager := kernel.ServiceAccountManager()

	// 1. CheckScope when no secret key in header
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	if !serviceAccountManager.CheckScope(listRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to return true when no secret key in header")
	}

	// 2. CheckScope with invalid secret key
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_secret_key")
	if serviceAccountManager.CheckScope(invalidKeyRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to return false for invalid secret key")
	}

	// 4. Test fallback path where PathValue is empty and path does not have user_id segment
	emptyUserPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/other", nil)
	emptyUserPathResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetUser(emptyUserPathResponseRecorder, emptyUserPathRequest)
	if emptyUserPathResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleGetUser with missing user ID in path, got: %d", emptyUserPathResponseRecorder.Code)
	}

	// 5. Test handleGetUser with broken DB on valid UUID -> 500
	userPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/01234567-89ab-cdef-0123-456789abcdef", nil)
	userPathRequest.SetPathValue("user_id", "01234567-89ab-cdef-0123-456789abcdef")
	userPathResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetUser(userPathResponseRecorder, userPathRequest)
	if userPathResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on handleGetUser with broken DB, got: %d", userPathResponseRecorder.Code)
	}

	// 6. Test handleGetConfig and handleUpdateConfig
	getConfigTestRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/config", nil)
	getConfigTestResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetConfig(getConfigTestResponseRecorder, getConfigTestRequest)
	if getConfigTestResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on handleGetConfig, got: %d", getConfigTestResponseRecorder.Code)
	}

	updateConfigTestRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader("{}"))
	updateConfigTestResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(updateConfigTestResponseRecorder, updateConfigTestRequest)
	if updateConfigTestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on handleUpdateConfig with broken pool, got: %d", updateConfigTestResponseRecorder.Code)
	}
}
