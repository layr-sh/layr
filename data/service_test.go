package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/realtime"
)

func TestDataServiceInitializationAndRoutesUnit(t *testing.T) {
	ctx := context.Background()
	dataService := NewService(nil)
	if dataService == nil {
		t.Fatal("expected non-nil service")
	}

	// 1. Dependency setters
	dataService.SetServiceAccountManager(nil)
	dataService.SetEventBus(nil)
	mockKVStore := newInMemoryKVStore()
	dataService.SetKVStore(mockKVStore)
	dataService.SetSaltSecret("test-salt-secret")

	// 2. Accessors
	if len(dataService.GetAllowedSchemas()) == 0 || !dataService.IsRESTEnabled() || dataService.GetRESTMaxLimit() != 1000 ||
		dataService.GetRESTDefaultLimit() != 50 || len(dataService.GetExcludedTables()) == 0 || !dataService.IsGraphQLEnabled() ||
		dataService.GetGraphQLMaxDepth() != 8 || !dataService.IsRealtimeEnabled() || dataService.GetRealtimeHeartbeatIntervalMS() != 30000 ||
		dataService.GetRealtimeMaxChannels() != 50 || len(dataService.GetAllowedOrigins()) == 0 {
		t.Fatal("unexpected accessor values from default service")
	}

	if dataService.DDLEngine() == nil || dataService.GetConfigManager() == nil || dataService.GetControlPlaneHandler() == nil ||
		dataService.GetRESTHandler() == nil || dataService.GetGraphQLHandler() == nil || dataService.GetRealtimeHandler() == nil ||
		dataService.GetRealtimeHub() == nil || dataService.BaseHandler() == nil || dataService.RealtimeHub() == nil ||
		dataService.GraphQLSchema() == nil {
		t.Fatal("expected non-nil sub-components from accessors")
	}

	// Nil service components test
	emptyService := &Service{}
	assert.Nil(t, emptyService.RealtimeHub())
	assert.Nil(t, emptyService.GraphQLSchema())
	assert.NoError(t, emptyService.IntrospectSchemas(ctx))
	assert.Nil(t, emptyService.GetRealtimeHub())
	emptyService.InvalidateTableCache(ctx, "public", "users")
	emptyService.ResetTableCacheVersion(ctx, "public", "users")
	emptyService.HandleFlushCache(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil))
	emptyService.HandleInvalidateCache(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil))

	dataService.ResetTableCacheVersion(ctx, "public", "users")

	// Non-nil introspect
	assert.NoError(t, dataService.IntrospectSchemas(ctx, "public"))
	assert.NoError(t, dataService.IntrospectSchemas(ctx))

	// 3. RegisterRoutes on Routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	dataService.RegisterRoutes(baseRouter, controlPlaneRouter)

	// 4. Scope check with nil serviceAccountManager
	scopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	if !dataService.CheckScope(scopeRequest, "data:query.read") {
		t.Fatal("expected CheckScope to return true when serviceAccountManager is nil")
	}
}

func TestDataServiceCacheInvalidationUnit(t *testing.T) {
	ctx := context.Background()
	dataService := NewService(nil)
	mockDriver := newInMemoryKVDriver()
	mockKVStore := core.NewKVStoreFromDriver(mockDriver)
	mockDriver.storage["cache:catalog"] = "catalog_data"
	mockDriver.storage["cache:schema:catalog"] = "schema_catalog"
	mockDriver.storage["cache:public.users"] = "user_cache"
	dataService.SetKVStore(mockKVStore)

	// 1. Invalidate single table cache
	dataService.InvalidateCache(ctx, InvalidateCacheRequest{Schema: "public", Table: "users"})
	if _, ok := mockDriver.storage["cache:public.users"]; ok {
		t.Fatal("expected table cache to be deleted")
	}
	if _, ok := mockDriver.storage["cache:catalog"]; !ok {
		t.Fatal("expected catalog cache to remain")
	}

	// 2. Invalidate Catalog via InvalidateCache
	dataService.InvalidateCache(ctx, InvalidateCacheRequest{Catalog: true})
	if _, ok := mockDriver.storage["cache:catalog"]; ok {
		t.Fatal("expected catalog cache to be deleted")
	}

	// 3. HandleFlushCache
	flushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/flush", nil)
	flushResponseRecorder := httptest.NewRecorder()
	dataService.HandleFlushCache(flushResponseRecorder, flushRequest)
	if flushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on HandleFlushCache, got %d", flushResponseRecorder.Code)
	}

	// 4. HandleInvalidateCache with valid JSON body
	invalidationReader := bytes.NewReader([]byte(`{"schema":"public","table":"products"}`))
	invalidationRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/invalidate", invalidationReader)
	invalidationResponseRecorder := httptest.NewRecorder()
	dataService.HandleInvalidateCache(invalidationResponseRecorder, invalidationRequest)
	if invalidationResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on HandleInvalidateCache, got %d", invalidationResponseRecorder.Code)
	}

	// 5. HandleInvalidateCache with malformed JSON
	malformedReader := bytes.NewReader([]byte(`{bad json`))
	malformedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/invalidate", malformedReader)
	malformedResponseRecorder := httptest.NewRecorder()
	dataService.HandleInvalidateCache(malformedResponseRecorder, malformedRequest)
	if malformedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed HandleInvalidateCache, got %d", malformedResponseRecorder.Code)
	}

	// 6. HandleInvalidateCache with nil body (empty)
	nilBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/invalidate", nil)
	nilBodyResponseRecorder := httptest.NewRecorder()
	dataService.HandleInvalidateCache(nilBodyResponseRecorder, nilBodyRequest)
	if nilBodyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on nil body HandleInvalidateCache, got %d", nilBodyResponseRecorder.Code)
	}

	// 8. Invalidate by pattern
	mockDriver.storage["cache:prefix:item"] = "cached"
	dataService.InvalidateCache(ctx, InvalidateCacheRequest{Pattern: "prefix:*"})
	assert.NotContains(t, mockDriver.storage, "cache:prefix:item")
}

func TestDataServiceScopeCheckUnit(t *testing.T) {
	dataService := NewService(nil)
	ctx := context.Background()

	// Start with nil db boots CDC and logs warning
	serviceConfig := dataService.GetConfigManager().Get()
	serviceConfig.Cache.Enabled = true
	serviceConfig.Cache.InvalidateOnCDC = true
	serviceConfig.Cache.CatalogTTLSeconds = 0
	dataService.GetConfigManager().SetMemoryConfig(serviceConfig)
	assert.NoError(t, dataService.Start(ctx))

	// Broadcast CDC event triggering InvalidateTableCache
	dataService.RealtimeHub().BroadcastEvent(realtime.CDCEvent{
		Schema: "public",
		Table:  "users",
		Event:  "UPDATE",
	})

	// Start with canceled context returns error
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	assert.Error(t, dataService.Start(canceledCtx))

	// Stop
	if err := dataService.Stop(); err != nil {
		t.Fatalf("unexpected error stopping unstarted service: %v", err)
	}
}
