// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerDomainsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	noScopeCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-none",
		JWT: core.JWTClaims{
			Subject:  "sa-none",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "unrelated:scope",
		},
	})

	var targetEndpoint Endpoint
	createEndpointErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
		VALUES ('cp-domain-test', 'workerd', 'index.js', 128, 30, true, clock_timestamp(), clock_timestamp())
		RETURNING id, name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at;
	`).Scan(&targetEndpoint.ID, &targetEndpoint.Name, &targetEndpoint.Runtime, &targetEndpoint.Entrypoint, &targetEndpoint.MemoryLimitMB, &targetEndpoint.TimeoutSeconds, &targetEndpoint.IsPublic, &targetEndpoint.CreatedAt, &targetEndpoint.UpdatedAt)
	require.NoError(t, createEndpointErr)

	// 1. Standalone Custom Domains
	// Scope denials
	unauthCreateDomainRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.test.com"}`))
	unauthCreateDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(unauthCreateDomainResponseRecorder, unauthCreateDomainRequest)
	require.Equal(t, http.StatusForbidden, unauthCreateDomainResponseRecorder.Code)

	unauthListDomainsRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/domains", nil)
	unauthListDomainsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomains(unauthListDomainsResponseRecorder, unauthListDomainsRequest)
	require.Equal(t, http.StatusForbidden, unauthListDomainsResponseRecorder.Code)

	unauthDeleteDomainRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/function/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	unauthDeleteDomainRequest.SetPathValue("id", "01923456-789a-7bc8-9def-0123456789ab")
	unauthDeleteDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(unauthDeleteDomainResponseRecorder, unauthDeleteDomainRequest)
	require.Equal(t, http.StatusForbidden, unauthDeleteDomainResponseRecorder.Code)

	// Validations
	badJSONDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`invalid-json`))
	badJSONDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(badJSONDomainResponseRecorder, badJSONDomainRequest)
	require.Equal(t, http.StatusBadRequest, badJSONDomainResponseRecorder.Code)

	invalidDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"-invalid-.com"}`))
	invalidDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(invalidDomainResponseRecorder, invalidDomainRequest)
	require.Equal(t, http.StatusBadRequest, invalidDomainResponseRecorder.Code)

	// Create valid domain
	validDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.test.com"}`))
	validDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(validDomainResponseRecorder, validDomainRequest)
	require.Equal(t, http.StatusCreated, validDomainResponseRecorder.Code)

	var customDomain CustomDomain
	require.NoError(t, json.Unmarshal(validDomainResponseRecorder.Body.Bytes(), &customDomain))
	require.Equal(t, "api.test.com", customDomain.Domain)

	// Domain conflict
	conflictDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.test.com"}`))
	conflictDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(conflictDomainResponseRecorder, conflictDomainRequest)
	require.Equal(t, http.StatusConflict, conflictDomainResponseRecorder.Code)

	// List domains with pagination
	listDomainsRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains?limit=10&offset=0", nil)
	listDomainsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomains(listDomainsResponseRecorder, listDomainsRequest)
	require.Equal(t, http.StatusOK, listDomainsResponseRecorder.Code)

	var listCustomDomainsResponse ListCustomDomainsResponse
	require.NoError(t, json.Unmarshal(listDomainsResponseRecorder.Body.Bytes(), &listCustomDomainsResponse))
	require.Equal(t, 1, listCustomDomainsResponse.Total)

	// Delete domain validations
	invalidDeleteIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/invalid-uuid", nil)
	invalidDeleteIDRequest.SetPathValue("id", "invalid-uuid")
	invalidDeleteIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(invalidDeleteIDResponseRecorder, invalidDeleteIDRequest)
	require.Equal(t, http.StatusBadRequest, invalidDeleteIDResponseRecorder.Code)

	notFoundDeleteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	notFoundDeleteRequest.SetPathValue("id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(notFoundDeleteResponseRecorder, notFoundDeleteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDeleteResponseRecorder.Code)

	// 2. Endpoint Domain Routes
	// Scope denials
	unauthCreateRouteRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{}`))
	unauthCreateRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	unauthCreateRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(unauthCreateRouteResponseRecorder, unauthCreateRouteRequest)
	require.Equal(t, http.StatusForbidden, unauthCreateRouteResponseRecorder.Code)

	unauthListRoutesRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", nil)
	unauthListRoutesRequest.SetPathValue("id", targetEndpoint.ID.String())
	unauthListRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(unauthListRoutesResponseRecorder, unauthListRoutesRequest)
	require.Equal(t, http.StatusForbidden, unauthListRoutesResponseRecorder.Code)

	unauthDeleteRouteRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	unauthDeleteRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	unauthDeleteRouteRequest.SetPathValue("route_id", "01923456-789a-7bc8-9def-0123456789ab")
	unauthDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(unauthDeleteRouteResponseRecorder, unauthDeleteRouteRequest)
	require.Equal(t, http.StatusForbidden, unauthDeleteRouteResponseRecorder.Code)

	// Route validations
	badEndpointRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/invalid-uuid/domains", strings.NewReader(`{}`))
	badEndpointRouteRequest.SetPathValue("id", "invalid-uuid")
	badEndpointRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(badEndpointRouteResponseRecorder, badEndpointRouteRequest)
	require.Equal(t, http.StatusBadRequest, badEndpointRouteResponseRecorder.Code)

	badJSONRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`invalid-json`))
	badJSONRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	badJSONRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(badJSONRouteResponseRecorder, badJSONRouteRequest)
	require.Equal(t, http.StatusBadRequest, badJSONRouteResponseRecorder.Code)

	missingDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{}`))
	missingDomainRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	missingDomainRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(missingDomainRouteResponseRecorder, missingDomainRouteRequest)
	require.Equal(t, http.StatusBadRequest, missingDomainRouteResponseRecorder.Code)

	notFoundEndpointRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/01923456-789a-7bc8-9def-0123456789ab/domains", strings.NewReader(`{"custom_domain_id":"`+customDomain.ID.String()+`"}`))
	notFoundEndpointRouteRequest.SetPathValue("id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundEndpointRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(notFoundEndpointRouteResponseRecorder, notFoundEndpointRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundEndpointRouteResponseRecorder.Code)

	notFoundDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{"custom_domain_id":"01923456-789a-7bc8-9def-0123456789ab"}`))
	notFoundDomainRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	notFoundDomainRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(notFoundDomainRouteResponseRecorder, notFoundDomainRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDomainRouteResponseRecorder.Code)

	// Invalid prefix route creation
	invalidPrefixRouteJSON := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/api?query=1"}`
	invalidPrefixRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(invalidPrefixRouteJSON))
	invalidPrefixRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	invalidPrefixRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(invalidPrefixRouteResponseRecorder, invalidPrefixRouteRequest)
	require.Equal(t, http.StatusBadRequest, invalidPrefixRouteResponseRecorder.Code)

	// Valid route creation
	createRouteJSON := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/api"}`
	validRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(createRouteJSON))
	validRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	validRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(validRouteResponseRecorder, validRouteRequest)
	require.Equal(t, http.StatusCreated, validRouteResponseRecorder.Code)

	var customDomainRoute CustomDomainRoute
	require.NoError(t, json.Unmarshal(validRouteResponseRecorder.Body.Bytes(), &customDomainRoute))
	require.Equal(t, "/api", customDomainRoute.PathPrefix)

	// Route conflict
	conflictRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(createRouteJSON))
	conflictRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	conflictRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(conflictRouteResponseRecorder, conflictRouteRequest)
	require.Equal(t, http.StatusConflict, conflictRouteResponseRecorder.Code)

	// List routes
	invalidEndpointListRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/invalid-uuid/domains", nil)
	invalidEndpointListRoutesRequest.SetPathValue("id", "invalid-uuid")
	invalidEndpointListRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(invalidEndpointListRoutesResponseRecorder, invalidEndpointListRoutesRequest)
	require.Equal(t, http.StatusBadRequest, invalidEndpointListRoutesResponseRecorder.Code)

	listRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", nil)
	listRoutesRequest.SetPathValue("id", targetEndpoint.ID.String())
	listRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
	require.Equal(t, http.StatusOK, listRoutesResponseRecorder.Code)

	var listCustomDomainRoutesResponse ListCustomDomainRoutesResponse
	require.NoError(t, json.Unmarshal(listRoutesResponseRecorder.Body.Bytes(), &listCustomDomainRoutesResponse))
	require.Equal(t, 1, listCustomDomainRoutesResponse.Total)

	// Delete route validations
	invalidEndpointDeleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/invalid-uuid/domains/invalid-uuid", nil)
	invalidEndpointDeleteRouteRequest.SetPathValue("id", "invalid-uuid")
	invalidEndpointDeleteRouteRequest.SetPathValue("route_id", "invalid-uuid")
	invalidEndpointDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(invalidEndpointDeleteRouteResponseRecorder, invalidEndpointDeleteRouteRequest)
	require.Equal(t, http.StatusBadRequest, invalidEndpointDeleteRouteResponseRecorder.Code)

	// Delete route validations
	invalidDeleteRouteIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/invalid-uuid", nil)
	invalidDeleteRouteIDRequest.SetPathValue("id", targetEndpoint.ID.String())
	invalidDeleteRouteIDRequest.SetPathValue("route_id", "invalid-uuid")
	invalidDeleteRouteIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(invalidDeleteRouteIDResponseRecorder, invalidDeleteRouteIDRequest)
	require.Equal(t, http.StatusBadRequest, invalidDeleteRouteIDResponseRecorder.Code)

	notFoundDeleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	notFoundDeleteRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	notFoundDeleteRouteRequest.SetPathValue("route_id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(notFoundDeleteRouteResponseRecorder, notFoundDeleteRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDeleteRouteResponseRecorder.Code)

	// Delete valid route
	deleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/"+customDomainRoute.ID.String(), nil)
	deleteRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	deleteRouteRequest.SetPathValue("route_id", customDomainRoute.ID.String())
	deleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(deleteRouteResponseRecorder, deleteRouteRequest)
	require.Equal(t, http.StatusNoContent, deleteRouteResponseRecorder.Code)

	// Insert route DB error (via trigger raising exception) (line 342)
	_, createRouteTrgErr := kernel.DB().Exec(testCtx, `
		CREATE OR REPLACE FUNCTION function.trg_fail_insert_route() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'simulated route insert failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER trg_test_fail_insert_route BEFORE INSERT ON function.custom_domain_routes
		FOR EACH ROW EXECUTE FUNCTION function.trg_fail_insert_route();
	`)
	require.NoError(t, createRouteTrgErr)

	failInsertRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{"custom_domain_id":"`+customDomain.ID.String()+`","path_prefix":"/fail-prefix"}`))
	failInsertRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	failInsertRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(failInsertRouteResponseRecorder, failInsertRouteRequest)
	require.Equal(t, http.StatusInternalServerError, failInsertRouteResponseRecorder.Code)

	_, dropRouteTrgErr := kernel.DB().Exec(testCtx, `DROP TRIGGER trg_test_fail_insert_route ON function.custom_domain_routes;`)
	require.NoError(t, dropRouteTrgErr)

	// Delete domain
	deleteDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/"+customDomain.ID.String(), nil)
	deleteDomainRequest.SetPathValue("id", customDomain.ID.String())
	deleteDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
	require.Equal(t, http.StatusNoContent, deleteDomainResponseRecorder.Code)

	// Fetch custom domain error during route creation (drop table custom_domains) -> 500 (line 300)
	_, dropCustomDomainsErr := kernel.DB().Exec(testCtx, `DROP TABLE function.custom_domains CASCADE;`)
	require.NoError(t, dropCustomDomainsErr)

	failFetchDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{"custom_domain_id":"`+customDomain.ID.String()+`","path_prefix":"/fail-fetch"}`))
	failFetchDomainRouteRequest.SetPathValue("id", targetEndpoint.ID.String())
	failFetchDomainRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(failFetchDomainRouteResponseRecorder, failFetchDomainRouteRequest)
	require.Equal(t, http.StatusInternalServerError, failFetchDomainRouteResponseRecorder.Code)
}

func TestFunctionControlPlaneHandlerDomainsDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	controlPlaneHandler := brokenService.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	randomUUID := "01923456-789a-7bc8-9def-0123456789ab"

	// Create domain DB failure -> 500
	createDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.test.com"}`))
	createDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(createDomainResponseRecorder, createDomainRequest)
	require.Equal(t, http.StatusInternalServerError, createDomainResponseRecorder.Code)

	// List domains DB failure -> 500
	listDomainsRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains", nil)
	listDomainsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomains(listDomainsResponseRecorder, listDomainsRequest)
	require.Equal(t, http.StatusInternalServerError, listDomainsResponseRecorder.Code)

	// Delete domain DB failure -> 500
	deleteDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/"+randomUUID, nil)
	deleteDomainRequest.SetPathValue("id", randomUUID)
	deleteDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
	require.Equal(t, http.StatusInternalServerError, deleteDomainResponseRecorder.Code)

	// Create route DB failure -> 500
	createRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+randomUUID+"/domains", strings.NewReader(`{"custom_domain_id":"`+randomUUID+`"}`))
	createRouteRequest.SetPathValue("id", randomUUID)
	createRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(createRouteResponseRecorder, createRouteRequest)
	require.Equal(t, http.StatusInternalServerError, createRouteResponseRecorder.Code)

	// List routes DB failure -> 500
	listRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+randomUUID+"/domains", nil)
	listRoutesRequest.SetPathValue("id", randomUUID)
	listRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
	require.Equal(t, http.StatusInternalServerError, listRoutesResponseRecorder.Code)

	// Delete route DB failure -> 500
	deleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+randomUUID+"/domains/"+randomUUID, nil)
	deleteRouteRequest.SetPathValue("id", randomUUID)
	deleteRouteRequest.SetPathValue("route_id", randomUUID)
	deleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(deleteRouteResponseRecorder, deleteRouteRequest)
	require.Equal(t, http.StatusInternalServerError, deleteRouteResponseRecorder.Code)

	// Fetch custom domain DB error
	_, fetchCustomDomainErr := controlPlaneHandler.fetchCustomDomainByID(rootCtx, uuid.NewV7())
	require.Error(t, fetchCustomDomainErr)
}

func TestFunctionNormalizeAndValidateDomainUnit(t *testing.T) {
	t.Parallel()

	// 1. Empty domain
	_, emptyDomainErr := NormalizeAndValidateDomain("   ")
	require.Error(t, emptyDomainErr)

	// 2. Host with valid port
	domainWithPort, domainWithPortErr := NormalizeAndValidateDomain("API.Example.COM:8080")
	require.NoError(t, domainWithPortErr)
	require.Equal(t, "api.example.com", domainWithPort)

	// 3. Host with invalid port syntax
	_, invalidPortSyntaxErr := NormalizeAndValidateDomain("example.com:80:90")
	require.Error(t, invalidPortSyntaxErr)

	// 4. Host with non-numeric port
	_, nonNumericPortErr := NormalizeAndValidateDomain("example.com:port")
	require.Error(t, nonNumericPortErr)

	// 5. Exceeding max domain length (253)
	tooLongDomain := strings.Repeat("a", 250) + ".com"
	_, maxDomainLengthErr := NormalizeAndValidateDomain(tooLongDomain)
	require.Error(t, maxDomainLengthErr)

	// 6. Fewer than 2 labels
	_, singleLabelDomainErr := NormalizeAndValidateDomain("localhost")
	require.Error(t, singleLabelDomainErr)

	// 7. Empty label
	_, emptyLabelDomainErr := NormalizeAndValidateDomain("api..example.com")
	require.Error(t, emptyLabelDomainErr)

	// 8. Label exceeding 63 chars
	tooLongLabel := strings.Repeat("a", 64) + ".com"
	_, labelTooLongErr := NormalizeAndValidateDomain(tooLongLabel)
	require.Error(t, labelTooLongErr)

	// 9. Invalid characters or hyphen placement
	_, invalidCharacterDomainErr := NormalizeAndValidateDomain("api.exam_ple.com")
	require.Error(t, invalidCharacterDomainErr)
	_, hyphenStartDomainErr := NormalizeAndValidateDomain("-api.example.com")
	require.Error(t, hyphenStartDomainErr)
}

func TestFunctionNormalizeAndValidatePathPrefixUnit(t *testing.T) {
	t.Parallel()

	// 1. Empty and root prefix
	emptyPrefix, emptyPrefixErr := NormalizeAndValidatePathPrefix("")
	require.NoError(t, emptyPrefixErr)
	require.Equal(t, "/", emptyPrefix)

	rootPrefix, rootPrefixErr := NormalizeAndValidatePathPrefix("  /  ")
	require.NoError(t, rootPrefixErr)
	require.Equal(t, "/", rootPrefix)

	// 2. Query param or fragment
	_, queryParamErr := NormalizeAndValidatePathPrefix("/api?version=1")
	require.Error(t, queryParamErr)

	_, fragmentErr := NormalizeAndValidatePathPrefix("/api#section")
	require.Error(t, fragmentErr)

	// 3. Whitespace characters
	_, whitespaceErr := NormalizeAndValidatePathPrefix("/api /v1")
	require.Error(t, whitespaceErr)

	// 4. Missing leading slash gets auto-prepended
	prependedPrefix, prependedPrefixErr := NormalizeAndValidatePathPrefix("api/v1")
	require.NoError(t, prependedPrefixErr)
	require.Equal(t, "/api/v1", prependedPrefix)

	// 5. Path length exceeding 255
	tooLongPath := "/" + strings.Repeat("a/", 130)
	_, pathLengthErr := NormalizeAndValidatePathPrefix(tooLongPath)
	require.Error(t, pathLengthErr)
}
