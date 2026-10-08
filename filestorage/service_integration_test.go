// Package filestorage encapsulates the File Storage service coordinator and handlers.
package filestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageServiceIntegration(t *testing.T) {
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
		Name:   "FileStorage Service Scope Verification Account",
		Scopes: []string{core.ScopeFileStorageBucketRead, core.ScopeFileStorageConfigRead},
	})
	require.NoError(t, err)

	// Authorized scope
	validScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/storage/buckets", nil)
	validScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	require.True(t, serviceAccountManager.CheckScope(validScopeRequest, core.ScopeFileStorageBucketRead))

	// Unauthorized scope
	missingScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/storage/buckets", nil)
	missingScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	require.False(t, serviceAccountManager.CheckScope(missingScopeRequest, core.ScopeFileStorageBucketWrite))

	// 4. Test ServiceFactory with core.Kernel
	serviceFactory, exists := core.GetServiceFactory("filestorage")
	require.True(t, exists)

	serviceRunner, factoryErr := serviceFactory(kernel)
	require.NoError(t, factoryErr)
	require.NotNil(t, serviceRunner)
}
