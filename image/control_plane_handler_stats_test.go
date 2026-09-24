package image

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerStatsUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	statsAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImageStatsRead,
		},
	}
	statsCtx := core.WithAuthContext(context.Background(), statsAuthContext)

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

	t.Run("missing scope returns 403", func(t *testing.T) {
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/image/stats", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)
	})

	t.Run("valid scope returns 200 with stats response", func(t *testing.T) {
		authedRequest := httptest.NewRequestWithContext(statsCtx, http.MethodGet, "/v1/_/image/stats", nil)
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(authedResponseRecorder, authedRequest)
		require.Equal(t, http.StatusOK, authedResponseRecorder.Code)

		var getStatsResponse GetStatsResponse
		unmarshalErr := json.Unmarshal(authedResponseRecorder.Body.Bytes(), &getStatsResponse)
		require.NoError(t, unmarshalErr)
		require.GreaterOrEqual(t, getStatsResponse.CacheHits, int64(0))
		require.GreaterOrEqual(t, getStatsResponse.CacheMisses, int64(0))
	})
}
