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
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	ctx := context.Background()

	// 1. Anonymous auth disabled -> 403
	disabledAnonymousConfig := DefaultConfig()
	disabledAnonymousConfig.Anonymous.Enabled = false
	configManager.Set(disabledAnonymousConfig)

	anonymousDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/anonymous", nil)
	anonymousDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAnonymousSignIn(anonymousDisabledResponseRecorder, anonymousDisabledRequest)
	if anonymousDisabledResponseRecorder.Code != http.StatusForbidden || !strings.Contains(anonymousDisabledResponseRecorder.Body.String(), "Access denied") {
		t.Fatalf("expected 403 on disabled anonymous auth, got: %d (%s)", anonymousDisabledResponseRecorder.Code, anonymousDisabledResponseRecorder.Body.String())
	}

	// 2. Anonymous auth enabled with nil database pool -> 500
	configManager.Set(DefaultConfig())
	anonymousNilDBRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/anonymous", strings.NewReader(`{"properties":{"source":"mobile"}}`))
	anonymousNilDBResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAnonymousSignIn(anonymousNilDBResponseRecorder, anonymousNilDBRequest)
	if anonymousNilDBResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on anonymous sign in with nil db, got: %d", anonymousNilDBResponseRecorder.Code)
	}
}
