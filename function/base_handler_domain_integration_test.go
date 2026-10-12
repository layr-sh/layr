package function

import (
	"context"
	"crypto/tls"
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

func TestFunctionBaseHandlerDomainTLSIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	// 1. Generate test TLS certificate for localhost
	certBytes := generateTestCertBytes(t)
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	// Configure TLS on core config
	coreConfig := core.DefaultConfig()
	coreConfig.Server.TLS.Enabled = true
	coreConfig.Server.TLS.AutoTLS = false
	coreConfig.Server.TLS.CertPEM = certPEM
	coreConfig.Server.TLS.KeyPEM = keyPEM
	coreConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(coreConfig)
	defer core.UnloadConfig()

	// 2. Provision public endpoint, custom domain, and route mapping
	publicEndpoint := insertTestEndpoint(testCtx, t, kernel, "tls-fn-endpoint", true)
	customDomain := insertTestCustomDomain(testCtx, t, kernel, "test.example.com")
	bindTestCustomDomainRoute(testCtx, t, kernel, publicEndpoint.ID, customDomain.ID, "/api")
	deployTestEndpoint(testCtx, t, service, &publicEndpoint, "export default { tls: true };")

	// 3. Setup core server with TLS and register routes
	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
	service.RegisterHostRoutes(coreServer)

	require.NotNil(t, coreServer.TLSConfig())
	require.NoError(t, coreServer.DBCertCache().Put(testCtx, "test.example.com", certBytes))

	// 4. Start HTTPS test server using coreServer handler and TLS config
	testServer := httptest.NewUnstartedServer(coreServer.Handler())
	testServer.TLS = coreServer.TLSConfig().Clone()
	testServer.StartTLS()
	defer testServer.Close()

	// 5. Execute HTTPS request to custom domain route
	tlsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
				ServerName:         "test.example.com",
			},
		},
	}

	httpsURL := testServer.URL + "/api/v1/test"
	httpRequest, requestErr := http.NewRequestWithContext(testCtx, http.MethodGet, httpsURL, nil)
	require.NoError(t, requestErr)
	httpRequest.Host = "test.example.com"

	httpResponse, doErr := tlsClient.Do(httpRequest)
	require.NoError(t, doErr)
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

	var execCount int
	require.NoError(t, kernel.DB().QueryRow(testCtx, "SELECT COUNT(*) FROM function.executions WHERE endpoint_id = $1;", publicEndpoint.ID).Scan(&execCount))
	require.Equal(t, 1, execCount)

	// 6. Test certificate provisioning status transition via handleCertPut
	service.handleCertPut("test.example.com", certBytes)

	var activeCustomDomain CustomDomain
	fetchErr := kernel.DB().QueryRow(testCtx, `
		SELECT id, domain, status, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at, tls_enabled
		FROM function.custom_domains
		WHERE id = $1;
	`, customDomain.ID).Scan(
		&activeCustomDomain.ID,
		&activeCustomDomain.Domain,
		&activeCustomDomain.Status,
		&activeCustomDomain.TLSStatus,
		&activeCustomDomain.TLSAcmeEmail,
		&activeCustomDomain.TLSErrorMessage,
		&activeCustomDomain.TLSIssuedAt,
		&activeCustomDomain.TLSExpiresAt,
		&activeCustomDomain.CreatedAt,
		&activeCustomDomain.UpdatedAt,
		&activeCustomDomain.TLSEnabled,
	)
	require.NoError(t, fetchErr)
	require.Equal(t, "active", activeCustomDomain.TLSStatus)
	require.True(t, activeCustomDomain.TLSEnabled)
	require.NotNil(t, activeCustomDomain.TLSIssuedAt)
	require.NotNil(t, activeCustomDomain.TLSExpiresAt)
	require.Nil(t, activeCustomDomain.TLSErrorMessage)
}
