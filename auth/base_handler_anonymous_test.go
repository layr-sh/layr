package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthHandlerAnonymousUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager
	ctx := context.Background()

	// 1. Anonymous auth disabled -> 403
	disabledAnonymousConfig := DefaultConfig()
	disabledAnonymousConfig.Anonymous.Enabled = false
	configManager.SetMemoryConfig(disabledAnonymousConfig)

	anonymousDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", nil)
	anonymousDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(anonymousDisabledResponseRecorder, anonymousDisabledRequest)
	if anonymousDisabledResponseRecorder.Code != http.StatusForbidden || !strings.Contains(anonymousDisabledResponseRecorder.Body.String(), "Access denied") {
		t.Fatalf("expected 403 on disabled anonymous auth, got: %d (%s)", anonymousDisabledResponseRecorder.Code, anonymousDisabledResponseRecorder.Body.String())
	}

	// 2. Anonymous auth enabled with broken database pool -> 500
	configManager.SetMemoryConfig(DefaultConfig())
	anonymousBrokenDBRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", strings.NewReader(`{"properties":{"source":"mobile"}}`))
	anonymousBrokenDBResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(anonymousBrokenDBResponseRecorder, anonymousBrokenDBRequest)
	if anonymousBrokenDBResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on anonymous sign in with broken db, got: %d", anonymousBrokenDBResponseRecorder.Code)
	}
}
