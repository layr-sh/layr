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
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	dataService := NewService(kernel)
	if dataService == nil {
		t.Fatal("expected non-nil service")
	}

	dataService.SetSaltSecret("test-salt-secret")

	// Accessors
	if len(dataService.GetAllowedSchemas()) == 0 || !dataService.IsRESTEnabled() || dataService.GetRESTMaxLimit() != 1000 ||
		dataService.GetRESTDefaultLimit() != 50 || len(dataService.GetExcludedTables()) == 0 || !dataService.IsGraphQLEnabled() ||
		dataService.GetGraphQLMaxDepth() != 8 || !dataService.IsRealtimeEnabled() || dataService.GetRealtimeHeartbeatIntervalMS() != 30000 ||
		dataService.GetRealtimeMaxChannels() != 50 || len(dataService.GetAllowedOrigins()) == 0 {
		t.Fatal("unexpected accessor values from default service")
	}

	if dataService.DDLEngine() == nil || dataService.ConfigManager() == nil || dataService.GetConfigManager() == nil ||
		dataService.ControlPlaneHandler() == nil || dataService.GetControlPlaneHandler() == nil ||
		dataService.GetRESTHandler() == nil || dataService.GetGraphQLHandler() == nil || dataService.GetRealtimeHandler() == nil ||
		dataService.GetRealtimeHub() == nil || dataService.BaseHandler() == nil || dataService.RealtimeHub() == nil ||
		dataService.GraphQLSchema() == nil || dataService.Kernel() == nil {
		t.Fatal("expected non-nil sub-components from accessors")
	}

	dataService.ResetTableCacheVersion(ctx, "public", "users")
	dataService.InvalidateTableCache(ctx, "public", "users")

	// Introspect
	assert.Error(t, dataService.IntrospectSchemas(ctx, "public"))
	assert.Error(t, dataService.IntrospectSchemas(ctx))

	// RegisterRoutes on Routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	dataService.RegisterRoutes(baseRouter, controlPlaneRouter)
}

func TestDataServiceCacheInvalidationUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	dataService := NewService(kernel)

	_ = kernel.KVStore().Set(ctx, "cache:catalog", "catalog_data", 0)
	_ = kernel.KVStore().Set(ctx, "cache:schema:catalog", "schema_catalog", 0)
	_ = kernel.KVStore().Set(ctx, "cache:public.users", "user_cache", 0)

	// 1. Invalidate single table cache
	dataService.InvalidateCache(ctx, InvalidateCacheInput{Schema: "public", Table: "users"})
	_, err := kernel.KVStore().Get(ctx, "cache:public.users")
	assert.Error(t, err)

	catalogValue, err := kernel.KVStore().Get(ctx, "cache:catalog")
	assert.NoError(t, err)
	assert.Equal(t, "catalog_data", catalogValue)

	// 2. Invalidate Catalog via InvalidateCache
	dataService.InvalidateCache(ctx, InvalidateCacheInput{Catalog: true})
	_, err = kernel.KVStore().Get(ctx, "cache:catalog")
	assert.Error(t, err)

	// 3. handleFlushCache
	flushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	flushResponseRecorder := httptest.NewRecorder()
	dataService.handleFlushCache(flushResponseRecorder, flushRequest)
	if flushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on handleFlushCache, got %d", flushResponseRecorder.Code)
	}

	// 4. handleInvalidateCache with valid JSON body
	invalidationReader := bytes.NewReader([]byte(`{"schema":"public","table":"products"}`))
	invalidationRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", invalidationReader)
	invalidationResponseRecorder := httptest.NewRecorder()
	dataService.handleInvalidateCache(invalidationResponseRecorder, invalidationRequest)
	if invalidationResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on handleInvalidateCache, got %d", invalidationResponseRecorder.Code)
	}

	// 5. handleInvalidateCache with malformed JSON
	malformedReader := bytes.NewReader([]byte(`{bad json`))
	malformedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", malformedReader)
	malformedResponseRecorder := httptest.NewRecorder()
	dataService.handleInvalidateCache(malformedResponseRecorder, malformedRequest)
	if malformedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed handleInvalidateCache, got %d", malformedResponseRecorder.Code)
	}

	// 6. handleInvalidateCache with nil body (empty)
	nilBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", nil)
	nilBodyResponseRecorder := httptest.NewRecorder()
	dataService.handleInvalidateCache(nilBodyResponseRecorder, nilBodyRequest)
	if nilBodyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on nil body handleInvalidateCache, got %d", nilBodyResponseRecorder.Code)
	}

	// 7. Invalidate by pattern
	_ = kernel.KVStore().Set(ctx, "cache:prefix:*", "cached", 0)
	dataService.InvalidateCache(ctx, InvalidateCacheInput{Pattern: "prefix:*"})
	_, err = kernel.KVStore().Get(ctx, "cache:prefix:*")
	assert.Error(t, err)
}

func TestDataServiceScopeCheckUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	dataService := NewService(kernel)
	ctx := context.Background()

	assert.NoError(t, dataService.GetConfigManager().Load(ctx))
	_, err := kernel.DB().Exec(ctx, `UPDATE data.config SET value = jsonb_set(value, '{cache,catalog_ttl_seconds}', '0') WHERE key = 'runtime'`)
	assert.NoError(t, err)

	assert.NoError(t, dataService.Start(ctx))

	// Broadcast CDC event triggering InvalidateTableCache in event handler
	dataService.RealtimeHub().BroadcastEvent(realtime.CDCEvent{
		Schema: "public",
		Table:  "users",
		Event:  "UPDATE",
	})

	// Broadcast when Cache.Enabled is false to cover false branch
	serviceConfig := dataService.GetConfigManager().Get()
	serviceConfig.Cache.Enabled = false
	dataService.GetConfigManager().SetMemoryConfig(serviceConfig)
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
	dataService.Stop()
}
