package tasks

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerConfigUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	tasksService := NewService(kernel)
	controlPlaneHandler := tasksService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	readConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksConfigRead,
		},
	}
	readConfigCtx := core.WithAuthContext(context.Background(), readConfigAuthContext)

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksConfigWrite,
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
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Valid scope -> 200
		validRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/tasks/config", nil)
		validResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(validResponseRecorder, validRequest)
		require.Equal(t, http.StatusOK, validResponseRecorder.Code)
	})

	t.Run("handleUpdateConfig unit tests", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/tasks/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Bad JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/tasks/config", bytes.NewReader([]byte("{bad")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Invalid validation config -> 400
		invalidPayload := []byte(`{"concurrency_limit":-5}`)
		invalidPayloadRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/tasks/config", bytes.NewReader(invalidPayload))
		invalidPayloadResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(invalidPayloadResponseRecorder, invalidPayloadRequest)
		require.Equal(t, http.StatusBadRequest, invalidPayloadResponseRecorder.Code)
	})
}
