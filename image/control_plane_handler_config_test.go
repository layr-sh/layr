package image

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerConfigUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	readConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImageConfigRead,
		},
	}
	readConfigCtx := core.WithAuthContext(context.Background(), readConfigAuthContext)

	writeConfigAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImageConfigWrite,
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
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/image/config", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Valid scope -> 200
		authedRequest := httptest.NewRequestWithContext(readConfigCtx, http.MethodGet, "/v1/_/image/config", nil)
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(authedResponseRecorder, authedRequest)
		require.Equal(t, http.StatusOK, authedResponseRecorder.Code)
	})

	t.Run("handle update config unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/image/config", bytes.NewReader([]byte(`{}`)))
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/image/config", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Valid update -> 200
		validPayload := `{"default_quality":85,"max_src_resolution":40,"max_animation_frames":64,"cache_ttl_seconds":3600,"watermark_opacity":0.6}`
		validUpdateRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/image/config", bytes.NewReader([]byte(validPayload)))
		validUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(validUpdateResponseRecorder, validUpdateRequest)
		require.Equal(t, http.StatusOK, validUpdateResponseRecorder.Code)

		// Invalid config validation failure (quality > 100) -> 500
		invalidPayload := `{"default_quality":150}`
		invalidConfigRequest := httptest.NewRequestWithContext(writeConfigCtx, http.MethodPut, "/v1/_/image/config", bytes.NewReader([]byte(invalidPayload)))
		invalidConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(invalidConfigResponseRecorder, invalidConfigRequest)
		require.Equal(t, http.StatusInternalServerError, invalidConfigResponseRecorder.Code)
	})
}
