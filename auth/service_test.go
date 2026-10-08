package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestAuthServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("auth")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(core.NewTestKernel(nil))
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		service, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, service.Kernel())
		require.NotNil(t, service.ConfigManager())
		require.NotNil(t, service.ControlPlaneHandler())
		require.NotNil(t, service.BaseHandler())
		require.NotNil(t, service.Hasher())
		require.NotNil(t, service.PasskeyManager())
		require.NotNil(t, service.TOTPManager())
		require.NotNil(t, service.EmailDispatcher())
		require.NotNil(t, service.SMSDispatcher())
		require.NotNil(t, service.HTTPClient())
		service.Stop()
	})

	t.Run("start returns error when context is canceled", func(t *testing.T) {
		t.Parallel()
		service := NewService(core.NewTestKernel(nil))
		require.NotNil(t, service)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startErr := service.Start(canceledCtx)
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "context canceled before start")

		service.Stop()
	})

	t.Run("start returns error with broken database", func(t *testing.T) {
		t.Parallel()
		kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		service := NewService(kernel)
		require.NotNil(t, service)

		startErr := service.Start(context.Background())
		require.Error(t, startErr)

		service.Stop()
	})

	t.Run("register routes with routers", func(t *testing.T) {
		t.Parallel()
		service := NewService(core.NewTestKernel(nil))
		require.NotNil(t, service)

		baseFuegoEngine := fuego.NewServer()
		controlPlaneFuegoEngine := fuego.NewServer()
		baseRouter := core.NewRouter(baseFuegoEngine)
		controlPlaneRouter := core.NewRouter(controlPlaneFuegoEngine)
		service.RegisterRoutes(baseRouter, controlPlaneRouter)
	})

	t.Run("check scope unit branches", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		service := NewService(kernel)

		// Branch A0: AuthContext is a service account -> evaluates HasScope directly
		m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
			ServiceAccountID: "sa_worker_123",
			JWT: core.JWTClaims{
				Subject: "sa_worker_123",
				Role:    "service_role",
				Scope:   core.ScopeAuthUserRead,
			},
		})
		m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/v1/test", nil)
		require.True(t, service.kernel.ServiceAccountManager().CheckScope(m2mValidRequest, core.ScopeAuthUserRead))
		require.False(t, service.kernel.ServiceAccountManager().CheckScope(m2mValidRequest, core.ScopeAuthUserWrite))

		// Branch B: serviceAccountManager present, but request has no service account key -> returns true
		noKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/test", nil)
		require.True(t, service.kernel.ServiceAccountManager().CheckScope(noKeyRequest, "auth:test.scope"))

		// Branch C: serviceAccountManager present, request has invalid secret key -> Authenticate fails -> returns false
		invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/test", nil)
		invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_secret_key_value")
		require.False(t, service.kernel.ServiceAccountManager().CheckScope(invalidKeyRequest, "auth:test.scope"))
	})
}
