package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
	"layr.sh/data/referencedata"
)

func TestDataServiceLifecycleIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	db := kernel.DB()
	serviceAccountManager := kernel.ServiceAccountManager()

	dataService := NewService(kernel)
	if startErr := dataService.Start(ctx); startErr != nil {
		t.Fatalf("failed to start data service: %v", startErr)
	}
	defer func() { dataService.Stop() }()

	// 1. Verify default dynamic configurations accessor values
	if !dataService.IsRESTEnabled() || dataService.GetRESTMaxLimit() <= 0 ||
		dataService.GetRESTDefaultLimit() <= 0 || len(dataService.GetExcludedTables()) == 0 || !dataService.IsGraphQLEnabled() ||
		dataService.GetGraphQLMaxDepth() <= 0 || !dataService.IsRealtimeEnabled() || dataService.GetRealtimeHeartbeatIntervalMS() <= 0 ||
		dataService.GetRealtimeMaxChannels() <= 0 {
		t.Fatal("unexpected config values returned by accessors")
	}

	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	dataService.RegisterRoutes(baseRouter, controlPlaneRouter)
	serveMux := controlPlaneRouter.Mux()

	// 2. Control Plane GET /v1/_/data/config
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	getResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(getResponseRecorder, getRequest)

	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected GET config 200, got %d: %s", getResponseRecorder.Code, getResponseRecorder.Body.String())
	}

	// 3. Control Plane PUT /v1/_/data/config with custom limits
	updatedConfig := DefaultConfig()
	updatedConfig.REST.MaxLimit = 500
	rawPut, marshalErr := json.Marshal(updatedConfig)
	if marshalErr != nil {
		t.Fatalf("failed to marshal config: %v", marshalErr)
	}
	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader(rawPut))
	putResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(putResponseRecorder, putRequest)

	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected PUT config 200, got %d: %s", putResponseRecorder.Code, putResponseRecorder.Body.String())
	}
	if dataService.GetRESTMaxLimit() != 500 {
		t.Fatalf("expected updated limit 500, got %d", dataService.GetRESTMaxLimit())
	}

	// 4. Test Load when row exists in DB
	if loadErr := dataService.GetConfigManager().Load(ctx); loadErr != nil {
		t.Fatalf("failed to load existing config: %v", loadErr)
	}

	// 5. Test Set validation rejects invalid config
	zeroConfig := Config{} // zero values for all
	if setErr := dataService.GetConfigManager().Set(ctx, zeroConfig); setErr == nil || !errors.Is(setErr, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig on zeroConfig, got: %v", setErr)
	}

	// 6. Test Load corrupted JSON error
	_, _ = db.Exec(ctx, "UPDATE data.config SET value = '\"corrupted string\"' WHERE key = 'runtime'")
	if corruptLoadErr := dataService.GetConfigManager().Load(ctx); corruptLoadErr == nil {
		t.Fatal("expected error loading corrupted json config")
	}

	// Restore valid config
	_ = dataService.GetConfigManager().Set(ctx, DefaultConfig())

	// 7. Control Plane Invalid JSON on PUT
	badPutRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader([]byte("not json")))
	badPutResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(badPutResponseRecorder, badPutRequest)
	if badPutResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad PUT json, got %d", badPutResponseRecorder.Code)
	}

	// 8. Test Cache Invalidation Endpoint POST /v1/_/data/cache/invalidate
	invalidTableRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"schema":"public","table":"users"}`)))
	invalidTableResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(invalidTableResponseRecorder, invalidTableRequest)
	if invalidTableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on invalidate table, got %d", invalidTableResponseRecorder.Code)
	}

	invalidCatalogRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"catalog":true}`)))
	invalidCatalogResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(invalidCatalogResponseRecorder, invalidCatalogRequest)
	if invalidCatalogResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on invalidate catalog, got %d", invalidCatalogResponseRecorder.Code)
	}

	invalidAllRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"all":true}`)))
	invalidAllResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(invalidAllResponseRecorder, invalidAllRequest)
	if invalidAllResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on invalidate all, got %d", invalidAllResponseRecorder.Code)
	}

	invalidBadJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte("invalid-json")))
	invalidBadJSONResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(invalidBadJSONResponseRecorder, invalidBadJSONRequest)
	if invalidBadJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON invalidate, got %d", invalidBadJSONResponseRecorder.Code)
	}

	// InvalidateCatalog
	dataService.InvalidateCatalog(ctx)
	dataService.SetSaltSecret("test-data-salt")
	_ = dataService.GetAllowedOrigins()

	// Error on Start with canceled context
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		brokenService := NewService(kernel)
		if startErr := brokenService.Start(canceledCtx); startErr == nil {
			t.Fatal("expected error starting service on canceled context")
		}
	}

	// 9. Test ServiceAccountManager, EventBus, and Cache Flush
	dummyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	dummyResponseRecorder := httptest.NewRecorder()
	if !serviceAccountManager.RequireScope(dummyResponseRecorder, dummyRequest, core.ScopeDataConfigRead) {
		t.Fatal("expected true when no service account key provided (internal/session allowed)")
	}
	if dataService.GetControlPlaneHandler() == nil {
		t.Fatal("expected non-nil control plane handler")
	}

	// Create a Service Account with data:cache.write
	controlPlaneCacheServiceAccount, createErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Cache Control Plane",
		Scopes: []string{core.ScopeDataCacheWrite},
	})
	if createErr != nil {
		t.Fatalf("failed to create service account: %v", createErr)
	}

	// Flush Cache with valid key
	flushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	flushRequest.Header.Set("Authorization", "Bearer "+controlPlaneCacheServiceAccount.SecretKey)
	flushResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(flushResponseRecorder, flushRequest)
	if flushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on flush cache, got %d: %s", flushResponseRecorder.Code, flushResponseRecorder.Body.String())
	}

	// Invalidate Cache with insufficient scope
	readOnlyServiceAccount, readOnlyErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only SA",
		Scopes: []string{core.ScopeDataQueryRead},
	})
	if readOnlyErr != nil {
		t.Fatalf("failed to create read-only service account: %v", readOnlyErr)
	}

	flushForbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	flushForbiddenRequest.Header.Set("Authorization", "Bearer "+readOnlyServiceAccount.SecretKey)
	flushForbiddenResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(flushForbiddenResponseRecorder, flushForbiddenRequest)
	if flushForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on flush cache with read-only scope, got %d", flushForbiddenResponseRecorder.Code)
	}

	invalidForbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", nil)
	invalidForbiddenRequest.Header.Set("Authorization", "Bearer "+readOnlyServiceAccount.SecretKey)
	invalidForbiddenResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(invalidForbiddenResponseRecorder, invalidForbiddenRequest)
	if invalidForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on invalidate cache with read-only scope, got %d", invalidForbiddenResponseRecorder.Code)
	}

	// Config endpoints with service account auth
	configForbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	configForbiddenRequest.Header.Set("Authorization", "Bearer "+readOnlyServiceAccount.SecretKey)
	configForbiddenResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(configForbiddenResponseRecorder, configForbiddenRequest)
	if configForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on GET config with read-only scope, got %d", configForbiddenResponseRecorder.Code)
	}

	configPutForbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader(rawPut))
	configPutForbiddenRequest.Header.Set("Authorization", "Bearer "+readOnlyServiceAccount.SecretKey)
	configPutForbiddenResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(configPutForbiddenResponseRecorder, configPutForbiddenRequest)
	if configPutForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on PUT config with read-only scope, got %d", configPutForbiddenResponseRecorder.Code)
	}

	// Bad service account key
	badKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	badKeyRequest.Header.Set("Authorization", "Bearer invalid_service_account_key_123")
	badKeyResponseRecorder := httptest.NewRecorder()
	serveMux.ServeHTTP(badKeyResponseRecorder, badKeyRequest)
	if badKeyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on bad service account key, got %d", badKeyResponseRecorder.Code)
	}
}

func TestDataServiceOpenAPIRoutesIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := kernel.ServiceAccountManager()

	dataService := NewService(kernel)

	if startErr := dataService.Start(ctx); startErr != nil {
		t.Fatalf("failed to start data service: %v", startErr)
	}
	defer func() {
		dataService.Stop()
	}()

	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())

	// Test nil safety
	dataService.RegisterRoutes(nil, nil)

	// Register actual routes
	dataService.RegisterRoutes(baseRouter, controlPlaneRouter)

	// Test OpenAPI spec endpoints contain Data service operations
	baseOpenAPISpec := baseRouter.OutputOpenAPISpec()
	if baseOpenAPISpec == nil {
		t.Fatal("expected non-nil public spec")
	}
	if baseOpenAPISpec.Paths.Value("/v1/data/{schema_name}/{table_name}") == nil ||
		baseOpenAPISpec.Paths.Value("/v1/graphql") == nil {
		t.Fatal("expected public spec to contain Data service paths")
	}

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	if controlPlaneOpenAPISpec == nil {
		t.Fatal("expected non-nil control plane spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/tables") == nil ||
		controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/sql") == nil {
		t.Fatal("expected control spec to contain Data service paths")
	}

	// Test invoking control plane config handler via OpenAPI router with scope check
	controlPlaneServiceAccount, createErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Config Control Plane",
		Scopes: []string{core.ScopeDataConfigRead, core.ScopeDataConfigWrite, core.ScopeDataCacheWrite},
	})
	if createErr != nil {
		t.Fatalf("failed to create control plane service account: %v", createErr)
	}

	// 1. GET /v1/_/data/config via control router
	configRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	configRequest.Header.Set("Authorization", "Bearer "+controlPlaneServiceAccount.SecretKey)
	configResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(configResponseRecorder, configRequest)
	if configResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET config via control router, got %d", configResponseRecorder.Code)
	}

	// 2. PUT /v1/_/data/config via control router
	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader([]byte(`{"rest":{"max_limit":200}}`)))
	putRequest.Header.Set("Authorization", "Bearer "+controlPlaneServiceAccount.SecretKey)
	putRequest.Header.Set("Content-Type", "application/json")
	putResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(putResponseRecorder, putRequest)
	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT config via control router, got %d", putResponseRecorder.Code)
	}

	// 3. POST /v1/_/data/cache/flush via control router
	flushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	flushRequest.Header.Set("Authorization", "Bearer "+controlPlaneServiceAccount.SecretKey)
	flushResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(flushResponseRecorder, flushRequest)
	if flushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST flush via control router, got %d", flushResponseRecorder.Code)
	}

	// 4. POST /v1/_/data/cache/invalidate via control router
	invalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"catalog":true}`)))
	invalidateRequest.Header.Set("Authorization", "Bearer "+controlPlaneServiceAccount.SecretKey)
	invalidateRequest.Header.Set("Content-Type", "application/json")
	invalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(invalidateResponseRecorder, invalidateRequest)
	if invalidateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST invalidate via control router, got %d", invalidateResponseRecorder.Code)
	}

	// 5. Test forbidden branches
	noScopeServiceAccount, noScopeErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "No Scope",
		Scopes: []string{"none"},
	})
	if noScopeErr != nil {
		t.Fatalf("failed to create no-scope service account: %v", noScopeErr)
	}

	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/_/data/config"},
		{http.MethodPut, "/v1/_/data/config"},
		{http.MethodPost, "/v1/_/data/cache/flush"},
		{http.MethodPost, "/v1/_/data/cache/invalidate"},
	} {
		t.Run(fmt.Sprintf("%s_%s", endpoint.method, endpoint.path), func(t *testing.T) {
			endpointRequest := httptest.NewRequestWithContext(ctx, endpoint.method, endpoint.path, nil)
			endpointRequest.Header.Set("Authorization", "Bearer "+noScopeServiceAccount.SecretKey)
			endpointResponseRecorder := httptest.NewRecorder()
			controlPlaneRouter.Mux().ServeHTTP(endpointResponseRecorder, endpointRequest)
			if endpointResponseRecorder.Code != http.StatusForbidden {
				t.Fatalf("expected 403 on %s %s with no scope, got %d", endpoint.method, endpoint.path, endpointResponseRecorder.Code)
			}
		})
	}

	// 6. Test Start failure when configManager.Load fails on closed database pool
	failingKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	failingService := NewService(failingKernel)
	if err := failingService.Start(ctx); err == nil {
		t.Fatal("expected error starting service with closed database pool")
	}
}

func TestDataServiceSeedErrorIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	restore := referencedata.SetEmbeddedJSONForTesting([]byte("invalid-json"), []byte("invalid-json"))
	defer restore()

	// Truncate reference_data.countries so fast-path check doesn't skip
	_, _ = kernel.DB().Exec(ctx, "TRUNCATE reference_data.countries CASCADE")

	dataService := NewService(kernel)
	if err := dataService.Start(ctx); err != nil {
		t.Fatalf("expected service.Start not to fail on seed error: %v", err)
	}
	defer func() { dataService.Stop() }()
}
