package function

import (
	"context"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"
)

func TestFunctionBaseHandlerCustomDomainUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	baseHandler := service.BaseHandler()

	// 1. Empty host or non-matching host returns false
	emptyHostRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/api/test", nil)
	emptyHostRequest.Host = ""
	emptyHostResponseRecorder := httptest.NewRecorder()
	require.False(t, baseHandler.HandleCustomDomain(emptyHostResponseRecorder, emptyHostRequest))

	unknownHostRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/api/test", nil)
	unknownHostRequest.Host = "unknown.example.com"
	unknownHostResponseRecorder := httptest.NewRecorder()
	require.False(t, baseHandler.HandleCustomDomain(unknownHostResponseRecorder, unknownHostRequest))

	// 2. Create public endpoint, bind custom domain, and deploy worker
	publicEndpoint := insertTestEndpoint(testCtx, t, kernel, "pub-domain-worker", true)
	createdPubCustomDomain := insertTestCustomDomain(testCtx, t, kernel, "api.myworker.com")
	bindTestCustomDomainRoute(testCtx, t, kernel, publicEndpoint.ID, createdPubCustomDomain.ID, "/")
	deployTestEndpoint(testCtx, t, service, &publicEndpoint, "export default { domain: true };")

	// 3. Invoke public custom domain with subpath and query parameters over HTTPS
	customDomainRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "https://api.myworker.com/v1/users?limit=10", nil)
	customDomainRequest.Host = "api.myworker.com"
	customDomainRequest.Header.Set("X-Forwarded-Proto", "https")
	customDomainResponseRecorder := httptest.NewRecorder()
	handled := baseHandler.HandleCustomDomain(customDomainResponseRecorder, customDomainRequest)
	require.True(t, handled)
	require.Equal(t, http.StatusOK, customDomainResponseRecorder.Code)

	// Verify execution is recorded in DB
	var execCount int
	require.NoError(t, kernel.DB().QueryRow(testCtx, "SELECT COUNT(*) FROM function.executions WHERE endpoint_id = $1;", publicEndpoint.ID).Scan(&execCount))
	require.Equal(t, 1, execCount)

	// 4. Test runner error during custom domain execution
	testRunner.shouldFail = true
	failingDomainRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/test", nil)
	failingDomainRequest.Host = "api.myworker.com:443"
	failingDomainResponseRecorder := httptest.NewRecorder()
	handledFail := baseHandler.HandleCustomDomain(failingDomainResponseRecorder, failingDomainRequest)
	require.True(t, handledFail)
	testRunner.shouldFail = false

	// 5. Custom domain with no active deployment returns 503
	undeployedEndpoint := insertTestEndpoint(testCtx, t, kernel, "undeployed-worker", true)
	createdUndeployedCustomDomain := insertTestCustomDomain(testCtx, t, kernel, "undeployed.myworker.com")
	bindTestCustomDomainRoute(testCtx, t, kernel, undeployedEndpoint.ID, createdUndeployedCustomDomain.ID, "/")

	undeployedRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/status", nil)
	undeployedRequest.Host = "undeployed.myworker.com"
	undeployedResponseRecorder := httptest.NewRecorder()
	handledUndeployed := baseHandler.HandleCustomDomain(undeployedResponseRecorder, undeployedRequest)
	require.True(t, handledUndeployed)
	require.Equal(t, http.StatusServiceUnavailable, undeployedResponseRecorder.Code)

	// 6. Private endpoint custom domain authorization
	privateEndpoint := insertTestEndpoint(testCtx, t, kernel, "priv-domain-worker", false)
	createdPrivCustomDomain := insertTestCustomDomain(testCtx, t, kernel, "secure.myworker.com")
	bindTestCustomDomainRoute(testCtx, t, kernel, privateEndpoint.ID, createdPrivCustomDomain.ID, "/")
	deployTestEndpoint(testCtx, t, service, &privateEndpoint, "export default { secure: true };")

	// Unauthenticated -> 401
	unauthPrivRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/profile", nil)
	unauthPrivRequest.Host = "secure.myworker.com"
	unauthPrivResponseRecorder := httptest.NewRecorder()
	handledUnauth := baseHandler.HandleCustomDomain(unauthPrivResponseRecorder, unauthPrivRequest)
	require.True(t, handledUnauth)
	require.Equal(t, http.StatusUnauthorized, unauthPrivResponseRecorder.Code)

	// Service account without scope -> 403
	forbiddenCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-unscoped",
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: "unrelated:scope",
		},
	})
	forbiddenRequest := httptest.NewRequestWithContext(forbiddenCtx, http.MethodGet, "/profile", nil)
	forbiddenRequest.Host = "secure.myworker.com"
	forbiddenResponseRecorder := httptest.NewRecorder()
	handledForbidden := baseHandler.HandleCustomDomain(forbiddenResponseRecorder, forbiddenRequest)
	require.True(t, handledForbidden)
	require.Equal(t, http.StatusForbidden, forbiddenResponseRecorder.Code)

	// Service account with scope -> 200
	authedCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-scoped",
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: core.ScopeFunctionEndpointInvoke,
		},
	})
	authedRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/profile", nil)
	authedRequest.Host = "secure.myworker.com"
	authedResponseRecorder := httptest.NewRecorder()
	handledAuthed := baseHandler.HandleCustomDomain(authedResponseRecorder, authedRequest)
	require.True(t, handledAuthed)
	require.Equal(t, http.StatusOK, authedResponseRecorder.Code)
}

func TestFunctionBaseHandlerResolveEndpointByCustomDomainUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	baseHandler := service.BaseHandler()

	// 1. Invalid domain host format error
	_, _, invalidHostErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "invalid host:99999", "/test")
	require.Error(t, invalidHostErr)

	// 2. Custom domain not found in DB
	_, _, notFoundErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "nonexistent.example.com", "/test")
	require.ErrorIs(t, notFoundErr, ErrCustomDomainNotFound)

	// 3. Custom domain exists, but route prefix does not match requested path
	createdCustomDomain := insertTestCustomDomain(testCtx, t, kernel, "prefix.myworker.com")
	targetEndpoint := insertTestEndpoint(testCtx, t, kernel, "prefix-endpoint", true)
	bindTestCustomDomainRoute(testCtx, t, kernel, targetEndpoint.ID, createdCustomDomain.ID, "/api")

	_, _, noPrefixMatchErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "prefix.myworker.com", "/nonmatching")
	require.ErrorIs(t, noPrefixMatchErr, ErrCustomDomainNotFound)

	// 4. Raw path without leading slash gets normalized with leading slash
	resolvedEndpoint, _, resolvePrefixErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "prefix.myworker.com", "api/users")
	require.NoError(t, resolvePrefixErr)
	require.Equal(t, targetEndpoint.ID, resolvedEndpoint.ID)

	// 5. Route points to deleted / non-existent endpoint
	missingEndpointID := uuid.NewV7()
	_, dropConstraintErr := kernel.DB().Exec(testCtx, `ALTER TABLE function.custom_domain_routes DROP CONSTRAINT IF EXISTS custom_domain_routes_endpoint_id_fkey;`)
	require.NoError(t, dropConstraintErr)
	const insertOrphanRouteSQL = `
		INSERT INTO function.custom_domain_routes (custom_domain_id, endpoint_id, path_prefix, created_at, updated_at)
		VALUES ($1, $2, '/orphan', clock_timestamp(), clock_timestamp());
	`
	_, insertOrphanErr := kernel.DB().Exec(testCtx, insertOrphanRouteSQL, createdCustomDomain.ID, missingEndpointID)
	require.NoError(t, insertOrphanErr)

	_, _, orphanErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "prefix.myworker.com", "/orphan")
	require.ErrorIs(t, orphanErr, ErrEndpointNotFound)

	// 5b. Custom domain with no routes returns ErrCustomDomainNotFound
	_ = insertTestCustomDomain(testCtx, t, kernel, "empty-routes.myworker.com")
	_, _, emptyRoutesErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "empty-routes.myworker.com", "/api")
	require.ErrorIs(t, emptyRoutesErr, ErrCustomDomainNotFound)

	// 5c. DB error when querying endpoint (drop table endpoints)
	_, dropEndpointsTableErr := kernel.DB().Exec(testCtx, `DROP TABLE function.endpoints CASCADE;`)
	require.NoError(t, dropEndpointsTableErr)
	_, _, endpointQueryFailErr := baseHandler.resolveEndpointByCustomDomain(testCtx, "prefix.myworker.com", "api/users")
	require.Error(t, endpointQueryFailErr)
	require.Contains(t, endpointQueryFailErr.Error(), "failed to query endpoint")

	// 6. DB error in queryDomainRoutesSQL
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	brokenBaseHandler := brokenService.BaseHandler()

	_, _, brokenDomainErr := brokenBaseHandler.resolveEndpointByCustomDomain(testCtx, "prefix.myworker.com", "/api")
	require.Error(t, brokenDomainErr)
	require.Contains(t, brokenDomainErr.Error(), "failed to fetch domain routes")

	// 7. fetchActiveDeployment when endpoint has no active deployment
	_, fetchActiveErr := baseHandler.fetchActiveDeployment(testCtx, uuid.NewV7())
	require.Error(t, fetchActiveErr)
}
