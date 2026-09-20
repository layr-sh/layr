package filestorage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerConfigUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil)
	controlPlaneHandler := NewControlPlaneHandler(nil, configManager, cryptoKeyManager)
	eventBus := core.NewEventBus(nil, cryptoKeyManager)
	controlPlaneHandler.SetEventBus(eventBus)

	readConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageConfigRead,
		},
	}
	readConfigCtx := core.WithAuthContext(context.Background(), readConfigAuthContext)

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageConfigWrite,
		},
	}
	writeConfigCtx := core.WithAuthContext(context.Background(), writeConfigAuthContext)

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

	t.Run("handle get config unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Valid scope -> 200
		authedRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/file-storage/config", nil)
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(authedResponseRecorder, authedRequest)
		if authedResponseRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for valid scope, got %d", authedResponseRecorder.Code)
		}

		// Nil config manager -> 500
		unconfiguredControlPlaneHandler := NewControlPlaneHandler(nil, nil, cryptoKeyManager)
		nilConfigRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/file-storage/config", nil)
		nilConfigResponseRecorder := httptest.NewRecorder()
		unconfiguredControlPlaneHandler.handleGetConfig(nilConfigResponseRecorder, nilConfigRequest)
		if nilConfigResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for nil config manager, got %d", nilConfigResponseRecorder.Code)
		}
	})

	t.Run("handle update config unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte(`{}`)))
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(badJSONResponseRecorder, badJSONRequest)
		if badJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for bad JSON, got %d", badJSONResponseRecorder.Code)
		}

		// Valid update -> 200
		validUpdateRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte(`{"default_max_file_size_bytes":10485760,"chunk_size_bytes":262144,"presign_token_expiry_seconds":1800}`)))
		validUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(validUpdateResponseRecorder, validUpdateRequest)
		if validUpdateResponseRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for valid update, got %d: %s", validUpdateResponseRecorder.Code, validUpdateResponseRecorder.Body.String())
		}

		// Invalid config validation failure (e.g. chunk_size_bytes <= 0) -> 500
		invalidConfigPayload := `{"default_max_file_size_bytes":1000,"chunk_size_bytes":-1}`
		invalidConfigRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte(invalidConfigPayload)))
		invalidConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(invalidConfigResponseRecorder, invalidConfigRequest)
		if invalidConfigResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for invalid config validation failure, got %d", invalidConfigResponseRecorder.Code)
		}

		// Nil config manager -> 500
		unconfiguredControlPlaneHandler := NewControlPlaneHandler(nil, nil, cryptoKeyManager)
		nilConfigRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte(`{}`)))
		nilConfigResponseRecorder := httptest.NewRecorder()
		unconfiguredControlPlaneHandler.handleUpdateConfig(nilConfigResponseRecorder, nilConfigRequest)
		if nilConfigResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for nil config manager, got %d", nilConfigResponseRecorder.Code)
		}
	})
}
