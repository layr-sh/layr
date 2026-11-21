package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestAuthControlPlaneHandlerStatsUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	for _, testCase := range []struct {
		name        string
		authContext core.AuthContext
		status      int
	}{
		{"unauthenticated", core.AuthContext{}, http.StatusForbidden},
		{"user scope does not grant stats", core.AuthContext{ServiceAccountID: "reader", JWT: core.JWTClaims{Scope: core.ScopeAuthUserRead}}, http.StatusForbidden},
		{"database unavailable", core.AuthContext{ServiceAccountID: "reader", JWT: core.JWTClaims{Scope: core.ScopeAuthStatsRead}}, http.StatusInternalServerError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			responseRecorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), testCase.authContext), http.MethodGet, "/v1/_/auth/stats", nil)
			service.controlPlaneHandler.handleGetStats(responseRecorder, request)
			require.Equal(t, testCase.status, responseRecorder.Code)
		})
	}
}
