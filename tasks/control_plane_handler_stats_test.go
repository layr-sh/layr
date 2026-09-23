package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerStatsUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	tasksService := NewService(kernel)
	controlPlaneHandler := tasksService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	statsAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksStatsRead,
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
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/stats", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)
	})

	t.Run("valid scope with broken database returns 500", func(t *testing.T) {
		authedRequest := httptest.NewRequestWithContext(statsCtx, http.MethodGet, "/v1/_/tasks/stats", nil)
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(authedResponseRecorder, authedRequest)
		require.Equal(t, http.StatusInternalServerError, authedResponseRecorder.Code)
	})
}
