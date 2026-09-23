// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksServiceIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Service with real kernel
	service := NewService(kernel)

	// 2. Start service (loads or seeds config in real DB)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	// 3. Create service account in DB and test CheckScope
	serviceAccountManager := kernel.ServiceAccountManager()
	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Tasks Service Scope Verification Account",
		Scopes: []string{core.ScopeTasksJobRead, core.ScopeTasksConfigRead},
	})
	require.NoError(t, err)

	// Authorized scope
	validScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs", nil)
	validScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	require.True(t, serviceAccountManager.CheckScope(validScopeRequest, core.ScopeTasksJobRead))

	// Unauthorized scope
	missingScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs", nil)
	missingScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	require.False(t, serviceAccountManager.CheckScope(missingScopeRequest, core.ScopeTasksJobWrite))

	// 4. Test ServiceFactory with core.Kernel
	serviceFactory, exists := core.GetServiceFactory("tasks")
	require.True(t, exists)

	serviceRunner, factoryErr := serviceFactory(kernel)
	require.NoError(t, factoryErr)
	require.NotNil(t, serviceRunner)
}
