// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

	initialCNAMEResolver := lookupCNAMEResolver
	defer func() {
		lookupCNAMEResolver = initialCNAMEResolver
	}()

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
	unauthDeleteDomainRequest.SetPathValue("domain_id", "01923456-789a-7bc8-9def-0123456789ab")
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

	// Conflict with core server base URL hostname and static TLS domains
	testConfig := core.DefaultConfig()
	testConfig.Server.BaseURL = "https://core.layr.sh:5297"
	testConfig.Server.TLS.Domains = []string{"static-core.layr.sh:443"}
	core.SetLoadedConfig(testConfig)

	coreConflictRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"core.layr.sh"}`))
	coreConflictResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(coreConflictResponseRecorder, coreConflictRequest)
	require.Equal(t, http.StatusBadRequest, coreConflictResponseRecorder.Code)
	require.Contains(t, coreConflictResponseRecorder.Body.String(), "domain conflicts with core server host")

	staticConflictRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"static-core.layr.sh"}`))
	staticConflictResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(staticConflictResponseRecorder, staticConflictRequest)
	require.Equal(t, http.StatusBadRequest, staticConflictResponseRecorder.Code)
	require.Contains(t, staticConflictResponseRecorder.Body.String(), "domain conflicts with core server host")
	core.UnloadConfig()

	// Create valid domain
	validDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.test.com"}`))
	validDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(validDomainResponseRecorder, validDomainRequest)
	require.Equal(t, http.StatusCreated, validDomainResponseRecorder.Code)

	var customDomain CustomDomain
	require.NoError(t, json.Unmarshal(validDomainResponseRecorder.Body.Bytes(), &customDomain))
	require.Equal(t, "api.test.com", customDomain.Domain)
	require.True(t, customDomain.TLSEnabled)
	require.Equal(t, "pending", customDomain.TLSStatus)
	require.Nil(t, customDomain.TLSAcmeEmail)
	require.NotEmpty(t, customDomain.RequiredDNSRecords)
	require.Equal(t, "CNAME", customDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "api", customDomain.RequiredDNSRecords[0].Name)
	require.Equal(t, "localhost", customDomain.RequiredDNSRecords[0].Target)
	require.Equal(t, "pending", customDomain.RequiredDNSRecords[0].Status)
	require.True(t, customDomain.RequiredDNSRecords[0].Required)

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
	require.NotEmpty(t, listCustomDomainsResponse.Data[0].RequiredDNSRecords)

	// Get domain validations and success
	unauthGetDomainRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/domains/"+customDomain.ID.String(), nil)
	unauthGetDomainRequest.SetPathValue("domain_id", customDomain.ID.String())
	unauthGetDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetCustomDomain(unauthGetDomainResponseRecorder, unauthGetDomainRequest)
	require.Equal(t, http.StatusForbidden, unauthGetDomainResponseRecorder.Code)

	badGetIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains/invalid-uuid", nil)
	badGetIDRequest.SetPathValue("domain_id", "invalid-uuid")
	badGetIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetCustomDomain(badGetIDResponseRecorder, badGetIDRequest)
	require.Equal(t, http.StatusBadRequest, badGetIDResponseRecorder.Code)

	notFoundGetRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	notFoundGetRequest.SetPathValue("domain_id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetCustomDomain(notFoundGetResponseRecorder, notFoundGetRequest)
	require.Equal(t, http.StatusNotFound, notFoundGetResponseRecorder.Code)

	validGetDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains/"+customDomain.ID.String(), nil)
	validGetDomainRequest.SetPathValue("domain_id", customDomain.ID.String())
	validGetDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetCustomDomain(validGetDomainResponseRecorder, validGetDomainRequest)
	require.Equal(t, http.StatusOK, validGetDomainResponseRecorder.Code)

	var fetchedCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(validGetDomainResponseRecorder.Body.Bytes(), &fetchedCustomDomain))
	require.Equal(t, customDomain.ID, fetchedCustomDomain.ID)
	require.Equal(t, "api.test.com", fetchedCustomDomain.Domain)
	require.NotEmpty(t, fetchedCustomDomain.RequiredDNSRecords)
	require.Equal(t, "CNAME", fetchedCustomDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "api", fetchedCustomDomain.RequiredDNSRecords[0].Name)

	// Test apex domain DNS instructions
	apexDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"apexdomain.com"}`))
	apexDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(apexDomainResponseRecorder, apexDomainRequest)
	require.Equal(t, http.StatusCreated, apexDomainResponseRecorder.Code)

	var apexCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(apexDomainResponseRecorder.Body.Bytes(), &apexCustomDomain))
	require.Len(t, apexCustomDomain.RequiredDNSRecords, 2)
	require.Equal(t, "A", apexCustomDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "@", apexCustomDomain.RequiredDNSRecords[0].Name)
	require.Equal(t, "127.0.0.1", apexCustomDomain.RequiredDNSRecords[0].Target)
	require.True(t, apexCustomDomain.RequiredDNSRecords[0].Required)
	require.Equal(t, "ALIAS", apexCustomDomain.RequiredDNSRecords[1].Type)
	require.Equal(t, "@", apexCustomDomain.RequiredDNSRecords[1].Name)
	require.Equal(t, "localhost", apexCustomDomain.RequiredDNSRecords[1].Target)
	require.False(t, apexCustomDomain.RequiredDNSRecords[1].Required)
	require.Contains(t, apexCustomDomain.RequiredDNSRecords[1].Comment, "ALIAS/ANAME")

	// Test apex domain with two-part ccTLD (company.co.uk)
	cctldApexDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"company.co.uk"}`))
	cctldApexDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(cctldApexDomainResponseRecorder, cctldApexDomainRequest)
	require.Equal(t, http.StatusCreated, cctldApexDomainResponseRecorder.Code)

	var cctldApexCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(cctldApexDomainResponseRecorder.Body.Bytes(), &cctldApexCustomDomain))
	require.Len(t, cctldApexCustomDomain.RequiredDNSRecords, 2)
	require.Equal(t, "A", cctldApexCustomDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "@", cctldApexCustomDomain.RequiredDNSRecords[0].Name)

	// Test subdomain with two-part ccTLD (api.company.co.uk)
	cctldSubDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"api.company.co.uk"}`))
	cctldSubDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(cctldSubDomainResponseRecorder, cctldSubDomainRequest)
	require.Equal(t, http.StatusCreated, cctldSubDomainResponseRecorder.Code)

	var cctldSubCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(cctldSubDomainResponseRecorder.Body.Bytes(), &cctldSubCustomDomain))
	require.Len(t, cctldSubCustomDomain.RequiredDNSRecords, 1)
	require.Equal(t, "CNAME", cctldSubCustomDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "api", cctldSubCustomDomain.RequiredDNSRecords[0].Name)

	// Test deep subdomain (deep.sub.company.com)
	deepSubDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"deep.sub.company.com"}`))
	deepSubDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(deepSubDomainResponseRecorder, deepSubDomainRequest)
	require.Equal(t, http.StatusCreated, deepSubDomainResponseRecorder.Code)

	var deepSubCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(deepSubDomainResponseRecorder.Body.Bytes(), &deepSubCustomDomain))
	require.Len(t, deepSubCustomDomain.RequiredDNSRecords, 1)
	require.Equal(t, "CNAME", deepSubCustomDomain.RequiredDNSRecords[0].Type)
	require.Equal(t, "deep.sub", deepSubCustomDomain.RequiredDNSRecords[0].Name)

	// Test custom Ingress target and Apex IP configured via configManager
	configuredConfig := service.configManager.Get()
	configuredConfig.CustomDomainCNAMETarget = "gateway.layr.sh"
	configuredConfig.CustomDomainApexIP = "198.51.100.25"
	service.configManager.config = configuredConfig

	configuredApexRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"configured.org"}`))
	configuredApexResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(configuredApexResponseRecorder, configuredApexRequest)
	require.Equal(t, http.StatusCreated, configuredApexResponseRecorder.Code)

	var configuredApexCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(configuredApexResponseRecorder.Body.Bytes(), &configuredApexCustomDomain))
	require.Len(t, configuredApexCustomDomain.RequiredDNSRecords, 2)
	require.Equal(t, "198.51.100.25", configuredApexCustomDomain.RequiredDNSRecords[0].Target)
	require.Equal(t, "gateway.layr.sh", configuredApexCustomDomain.RequiredDNSRecords[1].Target)

	// Delete domain validations
	invalidDeleteIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/invalid-uuid", nil)
	invalidDeleteIDRequest.SetPathValue("domain_id", "invalid-uuid")
	invalidDeleteIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(invalidDeleteIDResponseRecorder, invalidDeleteIDRequest)
	require.Equal(t, http.StatusBadRequest, invalidDeleteIDResponseRecorder.Code)

	notFoundDeleteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	notFoundDeleteRequest.SetPathValue("domain_id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(notFoundDeleteResponseRecorder, notFoundDeleteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDeleteResponseRecorder.Code)

	// Create domain with tls_enabled: false and tls_acme_email
	disabledTLSDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"disabled.test.com","tls_enabled":false,"tls_acme_email":"admin@disabled.test.com"}`))
	disabledTLSDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(disabledTLSDomainResponseRecorder, disabledTLSDomainRequest)
	require.Equal(t, http.StatusCreated, disabledTLSDomainResponseRecorder.Code)

	var disabledTLSCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(disabledTLSDomainResponseRecorder.Body.Bytes(), &disabledTLSCustomDomain))
	require.Equal(t, "disabled.test.com", disabledTLSCustomDomain.Domain)
	require.False(t, disabledTLSCustomDomain.TLSEnabled)
	require.Equal(t, "disabled", disabledTLSCustomDomain.TLSStatus)
	require.NotNil(t, disabledTLSCustomDomain.TLSAcmeEmail)
	require.Equal(t, "admin@disabled.test.com", *disabledTLSCustomDomain.TLSAcmeEmail)

	// Test that omitted TLSAcmeEmail keeps domain override nil, dynamically falling back to service default
	currentConfig := service.configManager.Get()
	currentConfig.DefaultTLSAcmeEmail = "default-fn@test.com"
	service.configManager.config = currentConfig

	defaultEmailDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"default-fallback.test.com"}`))
	defaultEmailDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(defaultEmailDomainResponseRecorder, defaultEmailDomainRequest)
	require.Equal(t, http.StatusCreated, defaultEmailDomainResponseRecorder.Code)

	var defaultEmailCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(defaultEmailDomainResponseRecorder.Body.Bytes(), &defaultEmailCustomDomain))
	require.Nil(t, defaultEmailCustomDomain.TLSAcmeEmail)
	require.Equal(t, "default-fn@test.com", controlPlaneHandler.resolveCustomDomainACMEEmail(&defaultEmailCustomDomain))

	// Test handleUpdateCustomDomain scope denial
	unauthUpdateDomainRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`{"tls_enabled":true}`))
	unauthUpdateDomainRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	unauthUpdateDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(unauthUpdateDomainResponseRecorder, unauthUpdateDomainRequest)
	require.Equal(t, http.StatusForbidden, unauthUpdateDomainResponseRecorder.Code)

	// Test handleUpdateCustomDomain invalid UUID
	badUpdateIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/invalid-uuid", strings.NewReader(`{"tls_enabled":true}`))
	badUpdateIDRequest.SetPathValue("domain_id", "invalid-uuid")
	badUpdateIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(badUpdateIDResponseRecorder, badUpdateIDRequest)
	require.Equal(t, http.StatusBadRequest, badUpdateIDResponseRecorder.Code)

	// Test handleUpdateCustomDomain invalid JSON
	badUpdateJSONRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`invalid-json`))
	badUpdateJSONRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	badUpdateJSONResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(badUpdateJSONResponseRecorder, badUpdateJSONRequest)
	require.Equal(t, http.StatusBadRequest, badUpdateJSONResponseRecorder.Code)

	// Test handleUpdateCustomDomain not found
	notFoundUpdateID := uuid.NewV7().String()
	notFoundUpdateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+notFoundUpdateID, strings.NewReader(`{"tls_enabled":true}`))
	notFoundUpdateRequest.SetPathValue("domain_id", notFoundUpdateID)
	notFoundUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(notFoundUpdateResponseRecorder, notFoundUpdateRequest)
	require.Equal(t, http.StatusNotFound, notFoundUpdateResponseRecorder.Code)

	// Test handleUpdateCustomDomain no-op update
	noopUpdateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`{}`))
	noopUpdateRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	noopUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(noopUpdateResponseRecorder, noopUpdateRequest)
	require.Equal(t, http.StatusOK, noopUpdateResponseRecorder.Code)

	// Test handleUpdateCustomDomain enable TLS: false -> true (tls_status -> pending)
	enableTLSRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`{"tls_enabled":true,"tls_acme_email":"new@disabled.test.com"}`))
	enableTLSRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	enableTLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(enableTLSResponseRecorder, enableTLSRequest)
	require.Equal(t, http.StatusOK, enableTLSResponseRecorder.Code)

	var enabledCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(enableTLSResponseRecorder.Body.Bytes(), &enabledCustomDomain))
	require.True(t, enabledCustomDomain.TLSEnabled)
	require.Equal(t, "pending", enabledCustomDomain.TLSStatus)
	require.NotNil(t, enabledCustomDomain.TLSAcmeEmail)
	require.Equal(t, "new@disabled.test.com", *enabledCustomDomain.TLSAcmeEmail)

	// Test handleUpdateCustomDomain reset tls_acme_email to empty
	resetEmailRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`{"tls_acme_email":""}`))
	resetEmailRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	resetEmailResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(resetEmailResponseRecorder, resetEmailRequest)
	require.Equal(t, http.StatusOK, resetEmailResponseRecorder.Code)

	var resetEmailCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(resetEmailResponseRecorder.Body.Bytes(), &resetEmailCustomDomain))
	require.Nil(t, resetEmailCustomDomain.TLSAcmeEmail)

	// Test handleUpdateCustomDomain disable TLS: true -> false (tls_status -> disabled)
	disableTLSRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), strings.NewReader(`{"tls_enabled":false}`))
	disableTLSRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	disableTLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(disableTLSResponseRecorder, disableTLSRequest)
	require.Equal(t, http.StatusOK, disableTLSResponseRecorder.Code)

	var reDisabledCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(disableTLSResponseRecorder.Body.Bytes(), &reDisabledCustomDomain))
	require.False(t, reDisabledCustomDomain.TLSEnabled)
	require.Equal(t, "disabled", reDisabledCustomDomain.TLSStatus)

	// Verify domain with TLSEnabled == false verifies DNS and keeps tls_status as disabled
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "gateway.layr.sh", nil
	}

	verifyDisabledTLSRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String()+"/verify", nil)
	verifyDisabledTLSRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	verifyDisabledTLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyDisabledTLSResponseRecorder, verifyDisabledTLSRequest)
	require.Equal(t, http.StatusOK, verifyDisabledTLSResponseRecorder.Code)

	var verifiedDisabledCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyDisabledTLSResponseRecorder.Body.Bytes(), &verifiedDisabledCustomDomain))
	require.Equal(t, "active", verifiedDisabledCustomDomain.Status)
	require.Equal(t, "disabled", verifiedDisabledCustomDomain.TLSStatus)

	// validateCustomDomainHost on domain with tls_enabled == false returns false
	validDisabledHost, validDisabledHostErr := service.validateCustomDomainHost(testCtx, "disabled.test.com")
	require.NoError(t, validDisabledHostErr)
	require.False(t, validDisabledHost)

	// Clean up disabled test domain
	deleteDisabledRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/"+disabledTLSCustomDomain.ID.String(), nil)
	deleteDisabledRequest.SetPathValue("domain_id", disabledTLSCustomDomain.ID.String())
	deleteDisabledResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteDisabledResponseRecorder, deleteDisabledRequest)
	require.Equal(t, http.StatusNoContent, deleteDisabledResponseRecorder.Code)

	// 2. Endpoint Domain Routes
	// Scope denials
	unauthCreateRouteRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{}`))
	unauthCreateRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	unauthCreateRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(unauthCreateRouteResponseRecorder, unauthCreateRouteRequest)
	require.Equal(t, http.StatusForbidden, unauthCreateRouteResponseRecorder.Code)

	unauthListRoutesRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", nil)
	unauthListRoutesRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	unauthListRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(unauthListRoutesResponseRecorder, unauthListRoutesRequest)
	require.Equal(t, http.StatusForbidden, unauthListRoutesResponseRecorder.Code)

	unauthDeleteRouteRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	unauthDeleteRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	unauthDeleteRouteRequest.SetPathValue("route_id", "01923456-789a-7bc8-9def-0123456789ab")
	unauthDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(unauthDeleteRouteResponseRecorder, unauthDeleteRouteRequest)
	require.Equal(t, http.StatusForbidden, unauthDeleteRouteResponseRecorder.Code)

	// Route validations
	badEndpointRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/invalid-uuid/domains", strings.NewReader(`{}`))
	badEndpointRouteRequest.SetPathValue("endpoint_id", "invalid-uuid")
	badEndpointRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(badEndpointRouteResponseRecorder, badEndpointRouteRequest)
	require.Equal(t, http.StatusBadRequest, badEndpointRouteResponseRecorder.Code)

	badJSONRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`invalid-json`))
	badJSONRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	badJSONRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(badJSONRouteResponseRecorder, badJSONRouteRequest)
	require.Equal(t, http.StatusBadRequest, badJSONRouteResponseRecorder.Code)

	missingDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{}`))
	missingDomainRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	missingDomainRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(missingDomainRouteResponseRecorder, missingDomainRouteRequest)
	require.Equal(t, http.StatusBadRequest, missingDomainRouteResponseRecorder.Code)

	notFoundEndpointRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/01923456-789a-7bc8-9def-0123456789ab/domains", strings.NewReader(`{"custom_domain_id":"`+customDomain.ID.String()+`"}`))
	notFoundEndpointRouteRequest.SetPathValue("endpoint_id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundEndpointRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(notFoundEndpointRouteResponseRecorder, notFoundEndpointRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundEndpointRouteResponseRecorder.Code)

	notFoundDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{"custom_domain_id":"01923456-789a-7bc8-9def-0123456789ab"}`))
	notFoundDomainRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	notFoundDomainRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(notFoundDomainRouteResponseRecorder, notFoundDomainRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDomainRouteResponseRecorder.Code)

	// Route creation on unverified domain returns 400 Bad Request
	createRouteJSON := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/api"}`
	unverifiedRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(createRouteJSON))
	unverifiedRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	unverifiedRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(unverifiedRouteResponseRecorder, unverifiedRouteRequest)
	require.Equal(t, http.StatusBadRequest, unverifiedRouteResponseRecorder.Code)

	// Verify the domain
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "gateway.layr.sh", nil
	}

	verifyDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+customDomain.ID.String()+"/verify", nil)
	verifyDomainRequest.SetPathValue("domain_id", customDomain.ID.String())
	verifyDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyDomainResponseRecorder, verifyDomainRequest)
	require.Equal(t, http.StatusOK, verifyDomainResponseRecorder.Code)

	// Valid route creation after domain verification
	validRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(createRouteJSON))
	validRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	validRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(validRouteResponseRecorder, validRouteRequest)
	require.Equal(t, http.StatusCreated, validRouteResponseRecorder.Code)

	var customDomainRoute CustomDomainRoute
	require.NoError(t, json.Unmarshal(validRouteResponseRecorder.Body.Bytes(), &customDomainRoute))
	require.Equal(t, "/api", customDomainRoute.PathPrefix)

	// Invalid prefix route creation on verified domain tests prefix validation failure
	invalidPrefixRouteJSON := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/api?query=1"}`
	invalidPrefixRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(invalidPrefixRouteJSON))
	invalidPrefixRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	invalidPrefixRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(invalidPrefixRouteResponseRecorder, invalidPrefixRouteRequest)
	require.Equal(t, http.StatusBadRequest, invalidPrefixRouteResponseRecorder.Code)

	// Route conflict
	conflictRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(createRouteJSON))
	conflictRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	conflictRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(conflictRouteResponseRecorder, conflictRouteRequest)
	require.Equal(t, http.StatusConflict, conflictRouteResponseRecorder.Code)

	// List routes
	invalidEndpointListRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/invalid-uuid/domains", nil)
	invalidEndpointListRoutesRequest.SetPathValue("endpoint_id", "invalid-uuid")
	invalidEndpointListRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(invalidEndpointListRoutesResponseRecorder, invalidEndpointListRoutesRequest)
	require.Equal(t, http.StatusBadRequest, invalidEndpointListRoutesResponseRecorder.Code)

	listRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", nil)
	listRoutesRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	listRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
	require.Equal(t, http.StatusOK, listRoutesResponseRecorder.Code)

	var listCustomDomainRoutesResponse ListCustomDomainRoutesResponse
	require.NoError(t, json.Unmarshal(listRoutesResponseRecorder.Body.Bytes(), &listCustomDomainRoutesResponse))
	require.Equal(t, 1, listCustomDomainRoutesResponse.Total)

	// Delete route validations
	invalidEndpointDeleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/invalid-uuid/domains/invalid-uuid", nil)
	invalidEndpointDeleteRouteRequest.SetPathValue("endpoint_id", "invalid-uuid")
	invalidEndpointDeleteRouteRequest.SetPathValue("route_id", "invalid-uuid")
	invalidEndpointDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(invalidEndpointDeleteRouteResponseRecorder, invalidEndpointDeleteRouteRequest)
	require.Equal(t, http.StatusBadRequest, invalidEndpointDeleteRouteResponseRecorder.Code)

	// Delete route validations
	invalidDeleteRouteIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/invalid-uuid", nil)
	invalidDeleteRouteIDRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	invalidDeleteRouteIDRequest.SetPathValue("route_id", "invalid-uuid")
	invalidDeleteRouteIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(invalidDeleteRouteIDResponseRecorder, invalidDeleteRouteIDRequest)
	require.Equal(t, http.StatusBadRequest, invalidDeleteRouteIDResponseRecorder.Code)

	notFoundDeleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/01923456-789a-7bc8-9def-0123456789ab", nil)
	notFoundDeleteRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	notFoundDeleteRouteRequest.SetPathValue("route_id", "01923456-789a-7bc8-9def-0123456789ab")
	notFoundDeleteRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomainRoute(notFoundDeleteRouteResponseRecorder, notFoundDeleteRouteRequest)
	require.Equal(t, http.StatusNotFound, notFoundDeleteRouteResponseRecorder.Code)

	// Delete valid route
	deleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/"+customDomainRoute.ID.String(), nil)
	deleteRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
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
	failInsertRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
	failInsertRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(failInsertRouteResponseRecorder, failInsertRouteRequest)
	require.Equal(t, http.StatusInternalServerError, failInsertRouteResponseRecorder.Code)

	_, dropRouteTrgErr := kernel.DB().Exec(testCtx, `DROP TRIGGER trg_test_fail_insert_route ON function.custom_domain_routes;`)
	require.NoError(t, dropRouteTrgErr)

	// Delete domain
	deleteDomainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/"+customDomain.ID.String(), nil)
	deleteDomainRequest.SetPathValue("domain_id", customDomain.ID.String())
	deleteDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
	require.Equal(t, http.StatusNoContent, deleteDomainResponseRecorder.Code)

	// Fetch custom domain error during route creation (drop table custom_domains) -> 500 (line 300)
	_, dropCustomDomainsErr := kernel.DB().Exec(testCtx, `DROP TABLE function.custom_domains CASCADE;`)
	require.NoError(t, dropCustomDomainsErr)

	failFetchDomainRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", strings.NewReader(`{"custom_domain_id":"`+customDomain.ID.String()+`","path_prefix":"/fail-fetch"}`))
	failFetchDomainRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
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
	deleteDomainRequest.SetPathValue("domain_id", randomUUID)
	deleteDomainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
	require.Equal(t, http.StatusInternalServerError, deleteDomainResponseRecorder.Code)

	// Create route DB failure -> 500
	createRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+randomUUID+"/domains", strings.NewReader(`{"custom_domain_id":"`+randomUUID+`"}`))
	createRouteRequest.SetPathValue("endpoint_id", randomUUID)
	createRouteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomainRoute(createRouteResponseRecorder, createRouteRequest)
	require.Equal(t, http.StatusInternalServerError, createRouteResponseRecorder.Code)

	// List routes DB failure -> 500
	listRoutesRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+randomUUID+"/domains", nil)
	listRoutesRequest.SetPathValue("endpoint_id", randomUUID)
	listRoutesResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
	require.Equal(t, http.StatusInternalServerError, listRoutesResponseRecorder.Code)

	// Delete route DB failure -> 500
	deleteRouteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+randomUUID+"/domains/"+randomUUID, nil)
	deleteRouteRequest.SetPathValue("endpoint_id", randomUUID)
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
	domainWithPort, domainWithPortErr := NormalizeAndValidateDomain("API.Example.COM:5297")
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

func TestFunctionControlPlaneHandlerTLSVerificationUnit(t *testing.T) {
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

	// 1. Create a custom domain in DB
	var customDomain CustomDomain
	createDomainErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_status, created_at, updated_at)
		VALUES ('nonexistent-unresolvable-domain-xyz987.test', 'active', 'pending', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`).Scan(&customDomain.ID, &customDomain.Domain, &customDomain.Status, &customDomain.TLSEnabled, &customDomain.TLSStatus, &customDomain.TLSAcmeEmail, &customDomain.TLSErrorMessage, &customDomain.TLSIssuedAt, &customDomain.TLSExpiresAt, &customDomain.CreatedAt, &customDomain.UpdatedAt)
	require.NoError(t, createDomainErr)
	require.Equal(t, "pending", customDomain.TLSStatus)
	require.True(t, customDomain.TLSEnabled)

	// 2. Validate custom domain host lookup
	validHost, hostErr := service.validateCustomDomainHost(testCtx, customDomain.Domain)
	require.NoError(t, hostErr)
	require.True(t, validHost)

	invalidHost, invalidErr := service.validateCustomDomainHost(testCtx, "completely-unknown.com")
	require.NoError(t, invalidErr)
	require.False(t, invalidHost)

	invalidSyntaxHost, invalidSyntaxErr := service.validateCustomDomainHost(testCtx, "invalid..domain")
	require.NoError(t, invalidSyntaxErr)
	require.False(t, invalidSyntaxHost)

	// 3. Scope denial on handleVerifyCustomDomain
	unauthRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/domains/"+customDomain.ID.String()+"/verify", nil)
	unauthRequest.SetPathValue("domain_id", customDomain.ID.String())
	unauthResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(unauthResponseRecorder, unauthRequest)
	require.Equal(t, http.StatusForbidden, unauthResponseRecorder.Code)

	// 4. Invalid domain UUID returns 400
	badIDRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/invalid-uuid/verify", nil)
	badIDRequest.SetPathValue("domain_id", "invalid-uuid")
	badIDResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(badIDResponseRecorder, badIDRequest)
	require.Equal(t, http.StatusBadRequest, badIDResponseRecorder.Code)

	// 5. Non-existent domain UUID returns 404
	randomID := uuid.NewV7().String()
	notFoundRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+randomID+"/verify", nil)
	notFoundRequest.SetPathValue("domain_id", randomID)
	notFoundResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(notFoundResponseRecorder, notFoundRequest)
	require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)

	// 6. Verify unresolvable domain fails DNS check -> tls_status = 'failed', status = 'pending'
	verifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+customDomain.ID.String()+"/verify", nil)
	verifyRequest.SetPathValue("domain_id", customDomain.ID.String())
	verifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyResponseRecorder, verifyRequest)
	require.Equal(t, http.StatusOK, verifyResponseRecorder.Code)

	var verifiedCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyResponseRecorder.Body.Bytes(), &verifiedCustomDomain))
	require.Equal(t, "failed", verifiedCustomDomain.TLSStatus)
	require.Equal(t, "pending", verifiedCustomDomain.Status)
	require.NotNil(t, verifiedCustomDomain.TLSErrorMessage)
	require.Contains(t, *verifiedCustomDomain.TLSErrorMessage, "DNS verification failed")

	// 7. Verify resolvable domain (localhost) succeeds DNS check -> tls_status = 'provisioning', status = 'active'
	var localhostCustomDomain CustomDomain
	createLocalhostErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_status, created_at, updated_at)
		VALUES ('localhost', 'active', 'pending', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`).Scan(&localhostCustomDomain.ID, &localhostCustomDomain.Domain, &localhostCustomDomain.Status, &localhostCustomDomain.TLSEnabled, &localhostCustomDomain.TLSStatus, &localhostCustomDomain.TLSAcmeEmail, &localhostCustomDomain.TLSErrorMessage, &localhostCustomDomain.TLSIssuedAt, &localhostCustomDomain.TLSExpiresAt, &localhostCustomDomain.CreatedAt, &localhostCustomDomain.UpdatedAt)
	require.NoError(t, createLocalhostErr)
	require.True(t, localhostCustomDomain.TLSEnabled)

	// 7a. Verify when server AutoTLS is not enabled on gateway -> tls_status = 'failed', tls_error_message = 'server AutoTLS is not enabled on this gateway'
	verifyLocalhostRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+localhostCustomDomain.ID.String()+"/verify", nil)
	verifyLocalhostRequest.SetPathValue("domain_id", localhostCustomDomain.ID.String())
	verifyLocalhostResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyLocalhostResponseRecorder, verifyLocalhostRequest)
	require.Equal(t, http.StatusOK, verifyLocalhostResponseRecorder.Code)

	var verifiedNoAutoTLSCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyLocalhostResponseRecorder.Body.Bytes(), &verifiedNoAutoTLSCustomDomain))
	require.Equal(t, "failed", verifiedNoAutoTLSCustomDomain.TLSStatus)
	require.Equal(t, "active", verifiedNoAutoTLSCustomDomain.Status)
	require.NotNil(t, verifiedNoAutoTLSCustomDomain.TLSErrorMessage)
	require.Equal(t, "server TLS is not enabled on this gateway", *verifiedNoAutoTLSCustomDomain.TLSErrorMessage)

	// 7a-2. Verify failing DNS on domain with tls_enabled=false preserves tls_status='disabled'
	var plainCustomDomain CustomDomain
	createPlainErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_enabled, tls_status, created_at, updated_at)
		VALUES ('plain-fail.test', 'pending', false, 'disabled', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, created_at, updated_at;
	`).Scan(&plainCustomDomain.ID, &plainCustomDomain.Domain, &plainCustomDomain.Status, &plainCustomDomain.TLSEnabled, &plainCustomDomain.TLSStatus, &plainCustomDomain.CreatedAt, &plainCustomDomain.UpdatedAt)
	require.NoError(t, createPlainErr)

	verifyPlainRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+plainCustomDomain.ID.String()+"/verify", nil)
	verifyPlainRequest.SetPathValue("domain_id", plainCustomDomain.ID.String())
	verifyPlainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyPlainResponseRecorder, verifyPlainRequest)
	require.Equal(t, http.StatusOK, verifyPlainResponseRecorder.Code)

	var verifiedPlainCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyPlainResponseRecorder.Body.Bytes(), &verifiedPlainCustomDomain))
	require.Equal(t, "disabled", verifiedPlainCustomDomain.TLSStatus, "failing DNS on tls_enabled=false domain must retain tls_status=disabled")
	require.Equal(t, "pending", verifiedPlainCustomDomain.Status)

	// 7b. Enable server AutoTLS and verify again -> tls_status = 'provisioning', status = 'active'
	autoTLSConfig := core.DefaultConfig()
	autoTLSConfig.Server.TLS.Enabled = true
	autoTLSConfig.Server.TLS.AutoTLS = true
	autoTLSConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(autoTLSConfig)
	defer core.UnloadConfig()

	autoTLSServer := core.NewServer(kernel)
	require.NotNil(t, autoTLSServer.AutocertManager())
	kernel.SetServer(autoTLSServer)

	verifyAutoTLSRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+localhostCustomDomain.ID.String()+"/verify", nil)
	verifyAutoTLSRequest.SetPathValue("domain_id", localhostCustomDomain.ID.String())
	verifyAutoTLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyAutoTLSResponseRecorder, verifyAutoTLSRequest)
	require.Equal(t, http.StatusOK, verifyAutoTLSResponseRecorder.Code)

	var verifiedLocalhostCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyAutoTLSResponseRecorder.Body.Bytes(), &verifiedLocalhostCustomDomain))
	require.Equal(t, "provisioning", verifiedLocalhostCustomDomain.TLSStatus)
	require.Equal(t, "active", verifiedLocalhostCustomDomain.Status)
	require.Nil(t, verifiedLocalhostCustomDomain.TLSErrorMessage)

	// 8. Test handleCertPut hook transitions domain to 'active'
	certBytes := generateTestCertBytes(t)
	service.handleCertPut("localhost", certBytes)

	// Fetch updated custom domain
	var activeCustomDomain CustomDomain
	fetchActiveErr := kernel.DB().QueryRow(testCtx, `
		SELECT id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at
		FROM function.custom_domains
		WHERE domain = 'localhost';
	`).Scan(&activeCustomDomain.ID, &activeCustomDomain.Domain, &activeCustomDomain.Status, &activeCustomDomain.TLSEnabled, &activeCustomDomain.TLSStatus, &activeCustomDomain.TLSAcmeEmail, &activeCustomDomain.TLSErrorMessage, &activeCustomDomain.TLSIssuedAt, &activeCustomDomain.TLSExpiresAt, &activeCustomDomain.CreatedAt, &activeCustomDomain.UpdatedAt)
	require.NoError(t, fetchActiveErr)
	require.Equal(t, "active", activeCustomDomain.TLSStatus)
	require.True(t, activeCustomDomain.TLSEnabled)
	require.NotNil(t, activeCustomDomain.TLSIssuedAt)
	require.NotNil(t, activeCustomDomain.TLSExpiresAt)
	require.Nil(t, activeCustomDomain.TLSErrorMessage)

	// 9. Edge cases for handleCertPut
	service.handleCertPut("localhost", nil) // No panic on nil certData
	service.handleCertPut("localhost", []byte("invalid-pem-data"))

	nonCertPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("dummy")})
	service.handleCertPut("localhost", nonCertPEM)

	invalidCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid-der")})
	service.handleCertPut("localhost", invalidCertPEM)

	// 10. RegisterHostRoutes
	coreServer := core.NewServer(kernel)
	service.RegisterHostRoutes(coreServer)

	autoTLSConfig = core.DefaultConfig()
	autoTLSConfig.Server.TLS.Enabled = true
	autoTLSConfig.Server.TLS.AutoTLS = true
	autoTLSConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(autoTLSConfig)
	defer core.UnloadConfig()

	autoTLSServer = core.NewServer(kernel)
	require.NotNil(t, autoTLSServer.DBCertCache())
	service.RegisterHostRoutes(autoTLSServer)

	origServer := kernel.Server()
	kernel.SetServer(autoTLSServer)
	controlPlaneHandler.triggerTLSProvisioning("autotls-test.com", "admin@autotls.com")
	kernel.SetServer(origServer)
}

func TestFunctionControlPlaneHandlerTLSErrorPathsUnit(t *testing.T) {
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

	// Create domains for update failure
	var unresolvableCustomDomain CustomDomain
	insertErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_status, created_at, updated_at)
		VALUES ('unresolvable-domain-trigger-test.test', 'active', 'pending', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`).Scan(&unresolvableCustomDomain.ID, &unresolvableCustomDomain.Domain, &unresolvableCustomDomain.Status, &unresolvableCustomDomain.TLSEnabled, &unresolvableCustomDomain.TLSStatus, &unresolvableCustomDomain.TLSAcmeEmail, &unresolvableCustomDomain.TLSErrorMessage, &unresolvableCustomDomain.TLSIssuedAt, &unresolvableCustomDomain.TLSExpiresAt, &unresolvableCustomDomain.CreatedAt, &unresolvableCustomDomain.UpdatedAt)
	require.NoError(t, insertErr)

	var resolvableCustomDomain CustomDomain
	insertResolvableErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_status, created_at, updated_at)
		VALUES ('localhost', 'active', 'pending', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`).Scan(&resolvableCustomDomain.ID, &resolvableCustomDomain.Domain, &resolvableCustomDomain.Status, &resolvableCustomDomain.TLSEnabled, &resolvableCustomDomain.TLSStatus, &resolvableCustomDomain.TLSAcmeEmail, &resolvableCustomDomain.TLSErrorMessage, &resolvableCustomDomain.TLSIssuedAt, &resolvableCustomDomain.TLSExpiresAt, &resolvableCustomDomain.CreatedAt, &resolvableCustomDomain.UpdatedAt)
	require.NoError(t, insertResolvableErr)

	// Add trigger to fail UPDATE on custom_domains
	_, triggerErr := kernel.DB().Exec(testCtx, `
		CREATE OR REPLACE FUNCTION fail_custom_domains_update_fn() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced update failure';
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER trg_fail_update
		BEFORE UPDATE ON function.custom_domains
		FOR EACH ROW EXECUTE FUNCTION fail_custom_domains_update_fn();
	`)
	require.NoError(t, triggerErr)

	// Test failed DNS resolution with DB update failure -> 500
	failVerifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+unresolvableCustomDomain.ID.String()+"/verify", nil)
	failVerifyRequest.SetPathValue("domain_id", unresolvableCustomDomain.ID.String())
	failVerifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(failVerifyResponseRecorder, failVerifyRequest)
	require.Equal(t, http.StatusInternalServerError, failVerifyResponseRecorder.Code)

	// Test successful DNS resolution with DB update failure -> 500
	successVerifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+resolvableCustomDomain.ID.String()+"/verify", nil)
	successVerifyRequest.SetPathValue("domain_id", resolvableCustomDomain.ID.String())
	successVerifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(successVerifyResponseRecorder, successVerifyRequest)
	require.Equal(t, http.StatusInternalServerError, successVerifyResponseRecorder.Code)

	// Test successful DNS resolution with DB update failure (AutoTLS enabled) -> 500
	failingAutoTLSConfig := core.DefaultConfig()
	failingAutoTLSConfig.Server.TLS.Enabled = true
	failingAutoTLSConfig.Server.TLS.AutoTLS = true
	failingAutoTLSConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(failingAutoTLSConfig)
	defer core.UnloadConfig()
	failingAutoTLSServer := core.NewServer(kernel)
	kernel.SetServer(failingAutoTLSServer)

	successVerifyAutoTLSRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+resolvableCustomDomain.ID.String()+"/verify", nil)
	successVerifyAutoTLSRequest.SetPathValue("domain_id", resolvableCustomDomain.ID.String())
	successVerifyAutoTLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(successVerifyAutoTLSResponseRecorder, successVerifyAutoTLSRequest)
	require.Equal(t, http.StatusInternalServerError, successVerifyAutoTLSResponseRecorder.Code)

	// Test handleUpdateCustomDomain with DB update failure -> 500
	failUpdateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+unresolvableCustomDomain.ID.String(), strings.NewReader(`{"tls_enabled":false}`))
	failUpdateRequest.SetPathValue("domain_id", unresolvableCustomDomain.ID.String())
	failUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(failUpdateResponseRecorder, failUpdateRequest)
	require.Equal(t, http.StatusInternalServerError, failUpdateResponseRecorder.Code)

	// Test broken DB kernel error paths
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	brokenControlPlaneHandler := brokenService.ControlPlaneHandler()

	_, hostErr := brokenService.validateCustomDomainHost(testCtx, "api.test.com")
	require.Error(t, hostErr)

	randomID := uuid.NewV7().String()
	brokenVerifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+randomID+"/verify", nil)
	brokenVerifyRequest.SetPathValue("domain_id", randomID)
	brokenVerifyResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleVerifyCustomDomain(brokenVerifyResponseRecorder, brokenVerifyRequest)
	require.Equal(t, http.StatusInternalServerError, brokenVerifyResponseRecorder.Code)

	brokenUpdateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+randomID, strings.NewReader(`{"tls_enabled":true}`))
	brokenUpdateRequest.SetPathValue("domain_id", randomID)
	brokenUpdateResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleUpdateCustomDomain(brokenUpdateResponseRecorder, brokenUpdateRequest)
	require.Equal(t, http.StatusInternalServerError, brokenUpdateResponseRecorder.Code)

	brokenGetRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/domains/"+randomID, nil)
	brokenGetRequest.SetPathValue("domain_id", randomID)
	brokenGetResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleGetCustomDomain(brokenGetResponseRecorder, brokenGetRequest)
	require.Equal(t, http.StatusInternalServerError, brokenGetResponseRecorder.Code)

	origTLSProvisionerFunc := tlsProvisionerFunc
	defer func() {
		tlsProvisionerFunc = origTLSProvisionerFunc
	}()

	provisionErrorSignal := make(chan struct{})
	tlsProvisionerFunc = func(ctx context.Context, server *core.Server, domain string, email string) error {
		defer close(provisionErrorSignal)
		return errors.New("simulated provision error")
	}

	controlPlaneHandler.triggerTLSProvisioning(unresolvableCustomDomain.Domain, "admin@test.com")
	<-provisionErrorSignal
	time.Sleep(30 * time.Millisecond)

	// Verify resolveCustomDomainACMEEmail
	customEmail := "custom@domain.com"
	emailCustomDomain := &CustomDomain{TLSAcmeEmail: &customEmail}
	require.Equal(t, "custom@domain.com", controlPlaneHandler.resolveCustomDomainACMEEmail(emailCustomDomain))

	service.configManager.config.DefaultTLSAcmeEmail = "default@domain.com"
	defaultEmailCustomDomain := &CustomDomain{}
	require.Equal(t, "default@domain.com", controlPlaneHandler.resolveCustomDomainACMEEmail(defaultEmailCustomDomain))

	service.configManager.config.DefaultTLSAcmeEmail = ""
	require.Empty(t, controlPlaneHandler.resolveCustomDomainACMEEmail(defaultEmailCustomDomain))

	// Test fallback to core config AcmeEmail
	coreFallbackConfig := core.DefaultConfig()
	coreFallbackConfig.Server.TLS.AcmeEmail = "core-fallback@layr.sh"
	core.SetLoadedConfig(coreFallbackConfig)
	require.Equal(t, "core-fallback@layr.sh", controlPlaneHandler.resolveCustomDomainACMEEmail(defaultEmailCustomDomain))
	core.UnloadConfig()

	// Test default tlsProvisioner error path when server or DBCertCache is nil
	require.Error(t, origTLSProvisionerFunc(context.Background(), nil, "test.example.com", "admin@test.com"))

	// Test default tlsProvisioner on nonAutoTLSServer (manual TLS mode) with uncached domain
	nonAutoTLSServer := core.NewServer(kernel)
	require.Error(t, origTLSProvisionerFunc(context.Background(), nonAutoTLSServer, "test.example.com", "admin@test.com"))

	// Test default tlsProvisioner success path with cached cert in server DBCertCache
	autoTLSConfig := core.DefaultConfig()
	autoTLSConfig.Server.TLS.Enabled = true
	autoTLSConfig.Server.TLS.AutoTLS = true
	autoTLSConfig.Server.TLS.Domains = []string{"test.example.com"}
	autoTLSConfig.Server.TLS.AcmeStaging = true
	autoTLSConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(autoTLSConfig)
	defer core.UnloadConfig()

	autoTLSServer := core.NewServer(kernel)
	require.NotNil(t, autoTLSServer.AutocertManager())
	require.NoError(t, autoTLSServer.DBCertCache().Put(context.Background(), "test.example.com", generateTestCertBytes(t)))
	require.NoError(t, origTLSProvisionerFunc(context.Background(), autoTLSServer, "test.example.com", "admin@test.com"))

	// Test default tlsProvisioner error path when cert is not in cache and lookup fails
	require.Error(t, origTLSProvisionerFunc(context.Background(), autoTLSServer, "uncached.invalid.test", ""))
}

func TestFunctionBuildRequiredDNSRecordsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	// 1. Status mappings
	recordsActive := controlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "active", "active")
	require.Equal(t, "verified", recordsActive[0].Status)

	recordsProvisioning := controlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "active", "provisioning")
	require.Equal(t, "verified", recordsProvisioning[0].Status)

	recordsFailed := controlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "pending", "failed")
	require.Equal(t, "misconfigured", recordsFailed[0].Status)

	recordsDisabled := controlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "pending", "disabled")
	require.Equal(t, "pending", recordsDisabled[0].Status)

	// 2. Handler without configManager
	withoutConfigControlPlaneHandler := &ControlPlaneHandler{Service: &Service{kernel: kernel}}
	recordsNoConfig := withoutConfigControlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "pending", "pending")
	require.NotEmpty(t, recordsNoConfig)
	require.Equal(t, "localhost", recordsNoConfig[0].Target)

	// 3. Fallback when Server.TLS.Domains is configured
	origConfig := core.GetConfig()
	customCoreConfig := *origConfig
	customCoreConfig.Server.TLS.Domains = []string{"tls-custom.example.com"}
	core.SetLoadedConfig(&customCoreConfig)
	defer core.SetLoadedConfig(origConfig)

	recordsTLSDomainsFallback := withoutConfigControlPlaneHandler.buildRequiredDNSRecords("sub.example.com", "pending", "pending")
	require.Equal(t, "tls-custom.example.com", recordsTLSDomainsFallback[0].Target)

	// 4. IP Target resolution when cnameTarget is an IP
	service.configManager.config.CustomDomainCNAMETarget = "10.0.0.1"
	service.configManager.config.CustomDomainApexIP = ""
	recordsIPTarget := controlPlaneHandler.buildRequiredDNSRecords("example.com", "pending", "pending")
	require.Len(t, recordsIPTarget, 2)
	require.Equal(t, "10.0.0.1", recordsIPTarget[0].Target)

	// 5. Hostname target for apex domain when apexIP is empty
	service.configManager.config.CustomDomainCNAMETarget = "ingress.cluster.com"
	recordsHostTarget := controlPlaneHandler.buildRequiredDNSRecords("example.com", "pending", "pending")
	require.Len(t, recordsHostTarget, 1)
	require.Equal(t, "ALIAS", recordsHostTarget[0].Type)
	require.Equal(t, "ingress.cluster.com", recordsHostTarget[0].Target)

	// 6. Deep ccTLD subdomain (deep.sub.example.co.uk)
	recordsDeepCCTLD := controlPlaneHandler.buildRequiredDNSRecords("deep.sub.example.co.uk", "pending", "pending")
	require.Len(t, recordsDeepCCTLD, 1)
	require.Equal(t, "CNAME", recordsDeepCCTLD[0].Type)
	require.Equal(t, "deep.sub", recordsDeepCCTLD[0].Name)

	// 7. Non-matching two-part TLD test
	require.False(t, isTwoPartCCTLD("unknown", "xyz"))

	// 8. Default CNAME resolver invocation
	_, _ = lookupCNAMEResolver(context.Background(), "localhost")
}

func TestFunctionVerifyCustomDomainDNSUnit(t *testing.T) {
	testCtx := context.Background()
	testControlPlaneHandler := &ControlPlaneHandler{}

	origCNAME := lookupCNAMEResolver
	origIP := lookupIPResolver
	defer func() {
		lookupCNAMEResolver = origCNAME
		lookupIPResolver = origIP
	}()

	// 1. Empty expected records
	matched, actual, _ := testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, nil)
	require.False(t, matched)
	require.Equal(t, "unconfigured", actual)

	// 2. CNAME direct match
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "gateway.layr.sh.", nil
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, []RequiredDNSRecord{
		{Type: "CNAME", Target: "gateway.layr.sh"},
	})
	require.True(t, matched)
	require.Equal(t, "gateway.layr.sh", actual)

	// 3. CNAME match via IP flattening
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "", errors.New("no cname")
	}
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == "sub.test.com" {
			return []net.IP{net.ParseIP("198.51.100.1")}, nil
		}
		if host == "gateway.layr.sh" {
			return []net.IP{net.ParseIP("198.51.100.1")}, nil
		}
		return nil, errors.New("unknown")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, []RequiredDNSRecord{
		{Type: "CNAME", Target: "gateway.layr.sh"},
	})
	require.True(t, matched)
	require.Equal(t, "198.51.100.1", actual)

	// 4. CNAME mismatch via IP
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == "sub.test.com" {
			return []net.IP{net.ParseIP("1.2.3.4")}, nil
		}
		if host == "gateway.layr.sh" {
			return []net.IP{net.ParseIP("198.51.100.1")}, nil
		}
		return nil, errors.New("unknown")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, []RequiredDNSRecord{
		{Type: "CNAME", Target: "gateway.layr.sh"},
	})
	require.False(t, matched)
	require.Equal(t, "1.2.3.4", actual)

	// 5. CNAME mismatch with different CNAME returned
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		return nil, errors.New("no ip")
	}
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "other.provider.com", nil
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, []RequiredDNSRecord{
		{Type: "CNAME", Target: "gateway.layr.sh"},
	})
	require.False(t, matched)
	require.Equal(t, "other.provider.com", actual)

	// 6. CNAME completely unresolvable
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "", errors.New("nxdomain")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "sub.test.com"}, []RequiredDNSRecord{
		{Type: "CNAME", Target: "gateway.layr.sh"},
	})
	require.False(t, matched)
	require.Equal(t, "unresolvable", actual)

	// 7. Apex A record match
	expectedApexRecords := []RequiredDNSRecord{
		{Type: "A", Target: "198.51.100.25"},
		{Type: "ALIAS", Target: "gateway.layr.sh"},
	}
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == "apex.com" {
			return []net.IP{net.ParseIP("198.51.100.25")}, nil
		}
		return nil, errors.New("unknown")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.True(t, matched)
	require.Equal(t, "198.51.100.25", actual)

	// 8. Apex match with ALIAS target IP
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == "apex.com" {
			return []net.IP{net.ParseIP("198.51.100.99")}, nil
		}
		if host == "gateway.layr.sh" {
			return []net.IP{net.ParseIP("198.51.100.99")}, nil
		}
		return nil, errors.New("unknown")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.True(t, matched)
	require.Equal(t, "198.51.100.99", actual)

	// 9. Apex IP mismatch
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		if host == "apex.com" {
			return []net.IP{net.ParseIP("9.9.9.9")}, nil
		}
		return []net.IP{net.ParseIP("198.51.100.99")}, nil
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.False(t, matched)
	require.Equal(t, "9.9.9.9", actual)

	// 10. Apex CNAME match with ALIAS target
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		return nil, errors.New("no ip")
	}
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "gateway.layr.sh", nil
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.True(t, matched)
	require.Equal(t, "gateway.layr.sh", actual)

	// 11. Apex CNAME mismatch
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "wrong.target.com", nil
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.False(t, matched)
	require.Equal(t, "wrong.target.com", actual)

	// 12. Apex unresolvable
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "", errors.New("nxdomain")
	}
	matched, actual, _ = testControlPlaneHandler.verifyCustomDomainDNS(testCtx, &CustomDomain{Domain: "apex.com"}, expectedApexRecords)
	require.False(t, matched)
	require.Equal(t, "unresolvable", actual)
}

func TestFunctionControlPlaneHandlerDomainValidationAndCleanupUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()
	coreServer := core.NewServer(kernel)

	rootCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	// 1. Create with invalid email returns 400 Bad Request
	invalidEmailCreateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"email-invalid.com","tls_acme_email":"notanemail"}`))
	invalidEmailCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(invalidEmailCreateResponseRecorder, invalidEmailCreateRequest)
	require.Equal(t, http.StatusBadRequest, invalidEmailCreateResponseRecorder.Code)
	require.Contains(t, invalidEmailCreateResponseRecorder.Body.String(), "email")

	// 2. Create valid domain
	validCreateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains", strings.NewReader(`{"domain":"cleanup-test.com","tls_acme_email":"admin@cleanup-test.com"}`))
	validCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateCustomDomain(validCreateResponseRecorder, validCreateRequest)
	require.Equal(t, http.StatusCreated, validCreateResponseRecorder.Code)

	var createdCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(validCreateResponseRecorder.Body.Bytes(), &createdCustomDomain))
	require.Equal(t, "cleanup-test.com", createdCustomDomain.Domain)
	require.NotNil(t, createdCustomDomain.TLSAcmeEmail)
	require.Equal(t, "admin@cleanup-test.com", *createdCustomDomain.TLSAcmeEmail)

	// 3. Update with invalid email returns 400 Bad Request
	invalidEmailUpdateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPatch, "/v1/_/function/domains/"+createdCustomDomain.ID.String(), strings.NewReader(`{"tls_acme_email":"bad-email-no-at"}`))
	invalidEmailUpdateRequest.SetPathValue("domain_id", createdCustomDomain.ID.String())
	invalidEmailUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateCustomDomain(invalidEmailUpdateResponseRecorder, invalidEmailUpdateRequest)
	require.Equal(t, http.StatusBadRequest, invalidEmailUpdateResponseRecorder.Code)
	require.Contains(t, invalidEmailUpdateResponseRecorder.Body.String(), "email")

	// 4. Seed certificate in core server DBCertCache
	require.NoError(t, coreServer.DBCertCache().Put(testCtx, createdCustomDomain.Domain, []byte("fake-cert-data")))
	cachedCert, certErr := coreServer.DBCertCache().Get(testCtx, createdCustomDomain.Domain)
	require.NoError(t, certErr)
	require.Equal(t, []byte("fake-cert-data"), cachedCert)

	// 5. Delete domain removes certificate from DBCertCache
	deleteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/domains/"+createdCustomDomain.ID.String(), nil)
	deleteRequest.SetPathValue("domain_id", createdCustomDomain.ID.String())
	deleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteCustomDomain(deleteResponseRecorder, deleteRequest)
	require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)

	// DBCertCache should now return autocert.ErrCacheMiss
	_, certFetchAfterDeleteErr := coreServer.DBCertCache().Get(testCtx, createdCustomDomain.Domain)
	require.Error(t, certFetchAfterDeleteErr)
}

func TestFunctionApexDomainRecognitionAndRecordsUnit(t *testing.T) {
	// 1. Test international ccTLDs
	testCases := []struct {
		domain       string
		expectedApex bool
		expectedPref string
	}{
		{"app.co.id", true, "@"},
		{"sub.app.co.id", false, "sub"},
		{"deep.sub.app.co.id", false, "deep.sub"},
		{"portal.com.sg", true, "@"},
		{"api.portal.com.sg", false, "api"},
		{"worker.co.in", true, "@"},
		{"api.worker.co.in", false, "api"},
		{"store.com.mx", true, "@"},
		{"tech.co.za", true, "@"},
		{"corp.com.tw", true, "@"},
		{"site.co.uk", true, "@"},
		{"api.site.co.uk", false, "api"},
		{"regular.com", true, "@"},
		{"api.regular.com", false, "api"},
	}

	for _, tc := range testCases {
		t.Run(tc.domain, func(t *testing.T) {
			isApex, prefix := extractDomainApexAndPrefix(tc.domain)
			require.Equal(t, tc.expectedApex, isApex, "apex check failed for %s", tc.domain)
			require.Equal(t, tc.expectedPref, prefix, "prefix check failed for %s", tc.domain)
		})
	}

	// 2. Test buildRequiredDNSRecords for apex domain without apex IP
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	// Configure cname target as hostname, apex IP empty
	customDomainConfig := DefaultConfig()
	customDomainConfig.CustomDomainCNAMETarget = "ingress.layr.sh"
	customDomainConfig.CustomDomainApexIP = ""
	service.configManager.SetMemoryConfig(customDomainConfig)

	records := controlPlaneHandler.buildRequiredDNSRecords("company.co.id", "pending", "pending")
	require.Len(t, records, 1)
	require.Equal(t, "ALIAS", records[0].Type)
	require.Equal(t, "ingress.layr.sh", records[0].Target)
	require.True(t, records[0].Required, "when apex IP is unconfigured, ALIAS record must be required")
}

func TestFunctionVerifyActiveCustomDomainIdempotentUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testConfig := core.DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = true
	testConfig.Server.TLS.AcmeEmail = "admin@layr.sh"
	testConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(testConfig)

	coreServer := core.NewServer(kernel)
	kernel.SetServer(coreServer)

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

	origCNAME := lookupCNAMEResolver
	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return "localhost", nil
	}
	defer func() {
		lookupCNAMEResolver = origCNAME
	}()

	// 1. Insert domain that is already active
	var activeCustomDomain CustomDomain
	insertErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_enabled, tls_status, created_at, updated_at)
		VALUES ('idempotent.active.com', 'active', true, 'active', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, created_at, updated_at;
	`).Scan(&activeCustomDomain.ID, &activeCustomDomain.Domain, &activeCustomDomain.Status, &activeCustomDomain.TLSEnabled, &activeCustomDomain.TLSStatus, &activeCustomDomain.CreatedAt, &activeCustomDomain.UpdatedAt)
	require.NoError(t, insertErr)

	// 2. Seed active certificate in DBCertCache
	require.NoError(t, coreServer.DBCertCache().Put(testCtx, activeCustomDomain.Domain, generateTestCertBytes(t)))

	// 3. Call verify on the active domain
	verifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+activeCustomDomain.ID.String()+"/verify", nil)
	verifyRequest.SetPathValue("domain_id", activeCustomDomain.ID.String())
	verifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(verifyResponseRecorder, verifyRequest)
	require.Equal(t, http.StatusOK, verifyResponseRecorder.Code)

	var verifiedCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(verifyResponseRecorder.Body.Bytes(), &verifiedCustomDomain))

	// Should remain active, NOT be reverted to provisioning
	require.Equal(t, "active", verifiedCustomDomain.Status)
	require.Equal(t, "active", verifiedCustomDomain.TLSStatus, "re-verifying an already active domain must preserve active tls_status")

	// 4. Pending domain with pre-existing certificate in cache verifies directly to active
	var pendingCustomDomain CustomDomain
	insertPendingErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_enabled, tls_status, created_at, updated_at)
		VALUES ('pending.precached.com', 'pending', true, 'pending', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, created_at, updated_at;
	`).Scan(&pendingCustomDomain.ID, &pendingCustomDomain.Domain, &pendingCustomDomain.Status, &pendingCustomDomain.TLSEnabled, &pendingCustomDomain.TLSStatus, &pendingCustomDomain.CreatedAt, &pendingCustomDomain.UpdatedAt)
	require.NoError(t, insertPendingErr)

	require.NoError(t, coreServer.DBCertCache().Put(testCtx, pendingCustomDomain.Domain, generateTestCertBytes(t)))

	pendingVerifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+pendingCustomDomain.ID.String()+"/verify", nil)
	pendingVerifyRequest.SetPathValue("domain_id", pendingCustomDomain.ID.String())
	pendingVerifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(pendingVerifyResponseRecorder, pendingVerifyRequest)
	require.Equal(t, http.StatusOK, pendingVerifyResponseRecorder.Code)

	var verifiedPendingCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(pendingVerifyResponseRecorder.Body.Bytes(), &verifiedPendingCustomDomain))
	require.Equal(t, "active", verifiedPendingCustomDomain.Status)
	require.Equal(t, "active", verifiedPendingCustomDomain.TLSStatus, "verifying pending domain with existing cert must immediately activate tls_status")

	// 4b. Re-verifying a domain with an EXPIRED certificate in cache MUST trigger renewal into provisioning status
	var expiredCustomDomain CustomDomain
	insertExpiredErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_enabled, tls_status, created_at, updated_at)
		VALUES ('expired.domain.com', 'active', true, 'active', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, created_at, updated_at;
	`).Scan(&expiredCustomDomain.ID, &expiredCustomDomain.Domain, &expiredCustomDomain.Status, &expiredCustomDomain.TLSEnabled, &expiredCustomDomain.TLSStatus, &expiredCustomDomain.CreatedAt, &expiredCustomDomain.UpdatedAt)
	require.NoError(t, insertExpiredErr)

	require.NoError(t, coreServer.DBCertCache().Put(testCtx, expiredCustomDomain.Domain, generateExpiredTestCertBytes(t)))

	origTLSProvisionerFunc := tlsProvisionerFunc
	tlsProvisionerFunc = func(ctx context.Context, server *core.Server, domain string, email string) error {
		return nil
	}
	defer func() {
		tlsProvisionerFunc = origTLSProvisionerFunc
	}()

	expiredVerifyRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/domains/"+expiredCustomDomain.ID.String()+"/verify", nil)
	expiredVerifyRequest.SetPathValue("domain_id", expiredCustomDomain.ID.String())
	expiredVerifyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleVerifyCustomDomain(expiredVerifyResponseRecorder, expiredVerifyRequest)
	require.Equal(t, http.StatusOK, expiredVerifyResponseRecorder.Code)

	var verifiedExpiredCustomDomain CustomDomain
	require.NoError(t, json.Unmarshal(expiredVerifyResponseRecorder.Body.Bytes(), &verifiedExpiredCustomDomain))
	require.Equal(t, "active", verifiedExpiredCustomDomain.Status)
	require.Equal(t, "provisioning", verifiedExpiredCustomDomain.TLSStatus, "re-verifying domain with expired cert in cache must trigger renewal into provisioning status")

	// 5. triggerTLSProvisioning success transitions provisioning domain to active
	var provCustomDomain CustomDomain
	insertProvErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.custom_domains (domain, status, tls_enabled, tls_status, created_at, updated_at)
		VALUES ('prov.success.com', 'active', true, 'provisioning', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, created_at, updated_at;
	`).Scan(&provCustomDomain.ID, &provCustomDomain.Domain, &provCustomDomain.Status, &provCustomDomain.TLSEnabled, &provCustomDomain.TLSStatus, &provCustomDomain.CreatedAt, &provCustomDomain.UpdatedAt)
	require.NoError(t, insertProvErr)

	controlPlaneHandler.triggerTLSProvisioning(provCustomDomain.Domain, "admin@layr.sh")
	require.Eventually(t, func() bool {
		fetchedCustomDomain, fetchErr := controlPlaneHandler.fetchCustomDomainByID(testCtx, provCustomDomain.ID)
		return fetchErr == nil && fetchedCustomDomain.TLSStatus == "active"
	}, 3*time.Second, 20*time.Millisecond)
}

func TestFunctionValidateTLSAcmeEmailUnit(t *testing.T) {
	require.Error(t, validateTLSAcmeEmail(""))
	require.Error(t, validateTLSAcmeEmail("   "))
	require.Error(t, validateTLSAcmeEmail("nodomain@"))
	require.Error(t, validateTLSAcmeEmail("@nodomain.com"))
	require.Error(t, validateTLSAcmeEmail("user@domainwithoutdot"))
	require.NoError(t, validateTLSAcmeEmail("user@domain.com"))
}

func TestFunctionTLSProvisionerCancellationUnit(t *testing.T) {
	testConfig := core.DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = true
	testConfig.Server.TLS.AcmeEmail = "admin@layr.sh"
	testConfig.Server.TLS.AcmeStaging = true
	testConfig.Security.MasterEncryptionKey = core.TestMasterEncryptionKeyHex
	core.SetLoadedConfig(testConfig)
	defer core.UnloadConfig()

	kernel := core.NewTestKernel(nil)
	server := core.NewServer(kernel)
	require.NotNil(t, server.AutocertManager())

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	provisionErr := tlsProvisionerFunc(canceledCtx, server, "example.com", "admin@layr.sh")
	require.Error(t, provisionErr)
	require.Contains(t, provisionErr.Error(), "canceled or timed out")
}

func TestFunctionNamespacedACMEAccountCacheUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	dbCertCache := kernel.Server().DBCertCache()
	require.NotNil(t, dbCertCache)

	// 1. With email: namespaced account key
	accountCache := &namespacedACMEAccountCache{
		email:           "tenant@example.com",
		underlyingCache: dbCertCache,
	}

	testCtx := context.Background()
	accountKey := "acme_account+key"
	accountData := []byte("secret-account-key")

	require.NoError(t, accountCache.Put(testCtx, accountKey, accountData))

	gotData, getErr := accountCache.Get(testCtx, accountKey)
	require.NoError(t, getErr)
	require.Equal(t, accountData, gotData)

	// Verify it was stored with the namespaced key in dbCertCache
	rawStoredData, rawErr := dbCertCache.Get(testCtx, "acme_account+key:tenant@example.com")
	require.NoError(t, rawErr)
	require.Equal(t, accountData, rawStoredData)

	// Domain cert key is not namespaced
	domainKey := "tenant.example.com"
	domainCertData := []byte("cert-payload")
	require.NoError(t, accountCache.Put(testCtx, domainKey, domainCertData))

	rawDomainData, rawDomainErr := dbCertCache.Get(testCtx, domainKey)
	require.NoError(t, rawDomainErr)
	require.Equal(t, domainCertData, rawDomainData)

	require.NoError(t, accountCache.Delete(testCtx, accountKey))
	require.NoError(t, accountCache.Delete(testCtx, domainKey))

	// 2. Without email: unnamespaced
	noEmailCache := &namespacedACMEAccountCache{
		email:           "",
		underlyingCache: dbCertCache,
	}
	require.NoError(t, noEmailCache.Put(testCtx, accountKey, accountData))
	rawNoEmailData, rawNoEmailErr := dbCertCache.Get(testCtx, accountKey)
	require.NoError(t, rawNoEmailErr)
	require.Equal(t, accountData, rawNoEmailData)
	require.NoError(t, noEmailCache.Delete(testCtx, accountKey))

	// 3. Error propagation from underlyingCache
	failingCache := &namespacedACMEAccountCache{
		email:           "tenant@example.com",
		underlyingCache: mockFailingACMEAccountCache{},
	}
	_, getErr = failingCache.Get(testCtx, accountKey)
	require.Error(t, getErr)
	require.Contains(t, getErr.Error(), "underlying get failed")

	putErr := failingCache.Put(testCtx, accountKey, accountData)
	require.Error(t, putErr)
	require.Contains(t, putErr.Error(), "failed to put cert data in cache")

	deleteErr := failingCache.Delete(testCtx, accountKey)
	require.Error(t, deleteErr)
	require.Contains(t, deleteErr.Error(), "failed to delete cert data from cache")
}

type mockFailingACMEAccountCache struct{}

func (mockFailingACMEAccountCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("underlying get failed")
}

func (mockFailingACMEAccountCache) Put(_ context.Context, _ string, _ []byte) error {
	return errors.New("underlying put failed")
}

func (mockFailingACMEAccountCache) Delete(_ context.Context, _ string) error {
	return errors.New("underlying delete failed")
}

func TestFunctionIsCertValidAndNotExpiringSoonUnit(t *testing.T) {
	// 1. Valid certificate with 90 days remaining
	validCert := generateTestCertBytes(t)
	require.True(t, isCertValidAndNotExpiringSoon(validCert))

	// 2. Expired certificate
	expiredCert := generateExpiredTestCertBytes(t)
	require.False(t, isCertValidAndNotExpiringSoon(expiredCert))

	// 3. No PEM blocks / empty bytes
	require.False(t, isCertValidAndNotExpiringSoon(nil))
	require.False(t, isCertValidAndNotExpiringSoon([]byte("invalid-pem-data")))

	// 4. PEM block that is not CERTIFICATE (e.g. only private key)
	onlyKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("test")})
	require.False(t, isCertValidAndNotExpiringSoon(onlyKeyPEM))

	// 5. Corrupt CERTIFICATE block
	corruptCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid-x509-der")})
	require.False(t, isCertValidAndNotExpiringSoon(corruptCertPEM))
}
