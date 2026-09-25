// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerConfigUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	readConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-config-read",
		JWT: core.JWTClaims{
			Subject:  "sa-config-read",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionConfigRead,
		},
	}
	readConfigCtx := core.WithAuthContext(testCtx, readConfigAuthContext)

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-config-write",
		JWT: core.JWTClaims{
			Subject:  "sa-config-write",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionConfigWrite,
		},
	}
	writeConfigCtx := core.WithAuthContext(testCtx, writeConfigAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-no-scope",
		JWT: core.JWTClaims{
			Subject:  "sa-no-scope",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(testCtx, noScopeAuthContext)

	t.Run("handleGetConfig permissions and output", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Authorized -> 200
		validRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/function/config", nil)
		validResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(validResponseRecorder, validRequest)
		require.Equal(t, http.StatusOK, validResponseRecorder.Code)

		var retrievedConfig Config
		require.NoError(t, json.Unmarshal(validResponseRecorder.Body.Bytes(), &retrievedConfig))
		require.Equal(t, 128, retrievedConfig.DefaultMemoryLimitMB)
	})

	t.Run("handleUpdateConfig validations and permissions", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/function/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Bad JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/function/config", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Invalid configuration values -> 400
		invalidConfig := service.ConfigManager().Get()
		invalidConfig.DefaultMemoryLimitMB = -1
		invalidConfigJSON, _ := json.Marshal(invalidConfig)
		invalidConfigRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/function/config", bytes.NewReader(invalidConfigJSON))
		invalidConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(invalidConfigResponseRecorder, invalidConfigRequest)
		require.Equal(t, http.StatusBadRequest, invalidConfigResponseRecorder.Code)

		// Valid configuration -> 200
		validConfig := service.ConfigManager().Get()
		validConfig.DefaultMemoryLimitMB = 256
		validConfigJSON, _ := json.Marshal(validConfig)
		validConfigRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/function/config", bytes.NewReader(validConfigJSON))
		validConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(validConfigResponseRecorder, validConfigRequest)
		require.Equal(t, http.StatusOK, validConfigResponseRecorder.Code)
	})
}

func TestFunctionControlPlaneHandlerConfigDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	controlPlaneHandler := brokenService.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionConfigWrite,
		},
	})

	updateConfigRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPut, "/v1/_/function/config", strings.NewReader(`{"runtimes":{"workerd":{"enabled":true}}}`))
	updateConfigResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(updateConfigResponseRecorder, updateConfigRequest)
	require.Equal(t, http.StatusInternalServerError, updateConfigResponseRecorder.Code)
}
