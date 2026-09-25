package function

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionBaseHandlerInvokeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// 1. Create and deploy public endpoint
	publicEndpoint := insertTestEndpoint(testCtx, t, kernel, "hello-integration-fn", true)
	deployTestEndpoint(testCtx, t, service, &publicEndpoint, "export default { hello: 'world' };")

	// 2. Invoke through coreServer
	request := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+publicEndpoint.Name+"/greet", nil)
	request.Header.Set("X-Layr-Client-Publishable-Key", kernel.CryptoKeyManager().DerivePublishableKey())
	responseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusOK, responseRecorder.Code)

	// 3. Verify execution was recorded in DB
	var execCount int
	require.NoError(t, kernel.DB().QueryRow(testCtx, "SELECT COUNT(*) FROM function.executions WHERE endpoint_id = $1;", publicEndpoint.ID).Scan(&execCount))
	require.Equal(t, 1, execCount)
}
