package function

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionBaseHandlerDomainIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	// 1. Create public endpoint, custom domain, and route mapping
	publicEndpoint := insertTestEndpoint(testCtx, t, kernel, "domain-integration-fn", true)
	customDomain := insertTestCustomDomain(testCtx, t, kernel, "integration.layr.app")
	bindTestCustomDomainRoute(testCtx, t, kernel, publicEndpoint.ID, customDomain.ID, "/api")
	deployTestEndpoint(testCtx, t, service, &publicEndpoint, "export default { integration: true };")

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
	service.RegisterHostRoutes(coreServer)

	// 2. Dispatch request to custom domain with path matching route prefix
	request := httptest.NewRequestWithContext(testCtx, http.MethodGet, "https://integration.layr.app/api/status", nil)
	request.Host = "integration.layr.app"
	request.Header.Set("X-Forwarded-Proto", "https")
	responseRecorder := httptest.NewRecorder()

	coreServer.Handler().ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusOK, responseRecorder.Code)

	// 3. Verify execution was recorded in DB
	var execCount int
	require.NoError(t, kernel.DB().QueryRow(testCtx, "SELECT COUNT(*) FROM function.executions WHERE endpoint_id = $1;", publicEndpoint.ID).Scan(&execCount))
	require.Equal(t, 1, execCount)
}
