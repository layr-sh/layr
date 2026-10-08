package console

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleControlPlaneHandlerConfigUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	consoleService := NewService(kernel)
	controlPlaneHandler := consoleService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	readConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeConsoleConfigRead,
		},
	}
	readConfigCtx := core.WithAuthContext(context.Background(), readConfigAuthContext)

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeConsoleConfigWrite,
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

	t.Run("handleGetConfig unit tests", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/console/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Valid scope -> 200
		validRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/console/config", nil)
		validResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(validResponseRecorder, validRequest)
		require.Equal(t, http.StatusOK, validResponseRecorder.Code)
	})

	t.Run("handleUpdateConfig unit tests", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/console/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Bad JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/console/config", bytes.NewReader([]byte("{bad")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Invalid validation config -> 400
		invalidPayload := []byte(`{"session_idle_timeout_seconds":-5}`)
		invalidPayloadRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/console/config", bytes.NewReader(invalidPayload))
		invalidPayloadResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(invalidPayloadResponseRecorder, invalidPayloadRequest)
		require.Equal(t, http.StatusBadRequest, invalidPayloadResponseRecorder.Code)

		// Valid update -> 200
		validPayload := []byte(`{"session_idle_timeout_seconds":900,"session_max_lifetime_seconds":3600,"audit_retention_days":30,"mfa_enforced":false,"allowed_cidrs":[]}`)
		validPayloadRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/console/config", bytes.NewReader(validPayload))
		validPayloadResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(validPayloadResponseRecorder, validPayloadRequest)
		require.Equal(t, http.StatusOK, validPayloadResponseRecorder.Code)
	})
}

func TestConsoleControlPlaneHandlerConfigDatabaseErrorUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	consoleService := NewService(kernel)
	controlPlaneHandler := consoleService.ControlPlaneHandler()

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeConsoleConfigWrite,
		},
	}
	writeConfigCtx := core.WithAuthContext(context.Background(), writeConfigAuthContext)

	validPayload := []byte(`{"session_idle_timeout_seconds":900,"session_max_lifetime_seconds":3600,"audit_retention_days":30,"mfa_enforced":false,"allowed_cidrs":[]}`)
	validRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/console/config", bytes.NewReader(validPayload))
	validResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(validResponseRecorder, validRequest)
	require.Equal(t, http.StatusInternalServerError, validResponseRecorder.Code)
}
