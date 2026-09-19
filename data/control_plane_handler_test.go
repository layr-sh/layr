package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerBaseConstructorAndSettersUnit(t *testing.T) {
	service := NewService(nil)
	controlPlaneHandler := NewControlPlaneHandler(service.ddlEngine, service, service.configManager)

	if controlPlaneHandler == nil {
		t.Fatal("expected non-nil controlPlaneHandler")
	}

	serviceAccountManager := core.NewServiceAccountManager(nil)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	if controlPlaneHandler.serviceAccountManager != serviceAccountManager {
		t.Fatal("expected serviceAccountManager to match")
	}

	eventBus := core.NewEventBus(nil, nil)
	controlPlaneHandler.SetEventBus(eventBus)
	if controlPlaneHandler.eventBus != eventBus {
		t.Fatal("expected eventBus to match")
	}

	controlPlaneHandler.SetKVStore(nil)
	if controlPlaneHandler.kvStore != nil {
		t.Fatal("expected kvStore to be nil")
	}
}

func TestDataControlPlaneHandlerBaseHelpersUnit(t *testing.T) {
	ctx := context.Background()
	service := NewService(nil)
	controlPlaneHandler := service.controlPlaneHandler

	// writeJSON
	jsonResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.writeJSON(jsonResponseRecorder, http.StatusOK, map[string]string{"status": "ok"})
	if jsonResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", jsonResponseRecorder.Code)
	}

	// writeError
	errorResponseRecorder := httptest.NewRecorder()
	errorRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/test", nil)
	core.WriteErrorResponse(errorResponseRecorder, errorRequest, http.StatusBadRequest, "bad request")
	if errorResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", errorResponseRecorder.Code)
	}

	// writeForbidden
	forbiddenResponseRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/test", nil)
	core.WriteErrorResponse(forbiddenResponseRecorder, forbiddenRequest, http.StatusForbidden, "Insufficient scope permissions for this operation")
	if forbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerBasePathExtractorsUnit(t *testing.T) {
	ctx := context.Background()
	controlPlaneHandler := NewControlPlaneHandler(nil, nil, nil)

	// Path Value extractor test
	pathValueRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/myschema/mytable/columns/mycolumn", nil)
	pathValueRequest.SetPathValue("schema_name", "myschema")
	pathValueRequest.SetPathValue("table_name", "mytable")
	pathValueRequest.SetPathValue("column_name", "mycolumn")
	pathValueRequest.SetPathValue("index_name", "myindex")
	pathValueRequest.SetPathValue("policy_name", "mypolicy")

	schema, table := controlPlaneHandler.extractSchemaAndTable(pathValueRequest)
	if schema != "myschema" || table != "mytable" {
		t.Fatalf("expected myschema/mytable, got %s/%s", schema, table)
	}
	if column := controlPlaneHandler.extractColumnName(pathValueRequest); column != "mycolumn" {
		t.Fatalf("expected mycolumn, got %s", column)
	}
	if index := controlPlaneHandler.extractIndexName(pathValueRequest); index != "myindex" {
		t.Fatalf("expected myindex, got %s", index)
	}
	if policy := controlPlaneHandler.extractPolicyName(pathValueRequest); policy != "mypolicy" {
		t.Fatalf("expected mypolicy, got %s", policy)
	}

	// URL path segments fallback test
	fallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/fallback_schema/fallback_table/columns/fallback_column", nil)
	schema, table = controlPlaneHandler.extractSchemaAndTable(fallbackRequest)
	if schema != "fallback_schema" || table != "fallback_table" {
		t.Fatalf("expected fallback_schema/fallback_table, got %s/%s", schema, table)
	}
	if column := controlPlaneHandler.extractColumnName(fallbackRequest); column != "fallback_column" {
		t.Fatalf("expected fallback_column, got %s", column)
	}

	indexFallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/fallback_schema/fallback_table/indexes/fallback_index", nil)
	if index := controlPlaneHandler.extractIndexName(indexFallbackRequest); index != "fallback_index" {
		t.Fatalf("expected fallback_index, got %s", index)
	}

	policyFallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/fallback_schema/fallback_table/policies/fallback_policy", nil)
	if policy := controlPlaneHandler.extractPolicyName(policyFallbackRequest); policy != "fallback_policy" {
		t.Fatalf("expected fallback_policy, got %s", policy)
	}

	// Empty / missing parts test
	emptyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables", nil)
	schema, table = controlPlaneHandler.extractSchemaAndTable(emptyRequest)
	if schema != "" || table != "" {
		t.Fatalf("expected empty schema/table, got %s/%s", schema, table)
	}
	if column := controlPlaneHandler.extractColumnName(emptyRequest); column != "" {
		t.Fatalf("expected empty column, got %s", column)
	}
	if index := controlPlaneHandler.extractIndexName(emptyRequest); index != "" {
		t.Fatalf("expected empty index, got %s", index)
	}
	if policy := controlPlaneHandler.extractPolicyName(emptyRequest); policy != "" {
		t.Fatalf("expected empty policy, got %s", policy)
	}

	// Test non-matching path with insufficient parts
	nonMatchingRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/public/users", nil)
	if index := controlPlaneHandler.extractIndexName(nonMatchingRequest); index != "" {
		t.Fatalf("expected empty index for non-matching path, got %s", index)
	}
	if policy := controlPlaneHandler.extractPolicyName(nonMatchingRequest); policy != "" {
		t.Fatalf("expected empty policy for non-matching path, got %s", policy)
	}
}

func TestDataControlPlaneHandlerBaseCheckScopeUnit(t *testing.T) {
	ctx := context.Background()

	// Nil manager returns true
	handlerWithoutManagerControlPlaneHandler := NewControlPlaneHandler(nil, nil, nil)
	allowedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/config", nil)
	if !handlerWithoutManagerControlPlaneHandler.checkScope(allowedRequest, "data:config.read") {
		t.Fatal("expected true when serviceAccountManager is nil")
	}

	// With service and nil manager
	service := NewService(nil)
	service.SetServiceAccountManager(nil)
	handlerWithServiceControlPlaneHandler := NewControlPlaneHandler(nil, service, nil)
	if !handlerWithServiceControlPlaneHandler.checkScope(allowedRequest, "data:config.read") {
		t.Fatal("expected true when service has nil serviceAccountManager")
	}

	// Empty key with serviceAccountManager returns true (internal / session auth)
	serviceAccountManager := core.NewServiceAccountManager(nil)
	handlerWithManagerControlPlaneHandler := NewControlPlaneHandler(nil, nil, nil)
	handlerWithManagerControlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	if !handlerWithManagerControlPlaneHandler.checkScope(allowedRequest, "data:config.read") {
		t.Fatal("expected true on empty service account key")
	}

	// Invalid key with serviceAccountManager returns false
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/config", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_key")
	if handlerWithManagerControlPlaneHandler.checkScope(invalidKeyRequest, "data:config.read") {
		t.Fatal("expected false on invalid service account key")
	}

	// Service role JWT with scope returns true
	allowedJWTRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: "data:config.read",
		},
	}), http.MethodGet, "/api/v1/_/data/config", nil)
	if !handlerWithManagerControlPlaneHandler.checkScope(allowedJWTRequest, "data:config.read") {
		t.Fatal("expected true on service_role JWT with required scope")
	}

	// Service role JWT without scope returns false
	deniedJWTRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: "other:scope",
		},
	}), http.MethodGet, "/api/v1/_/data/config", nil)
	if handlerWithManagerControlPlaneHandler.checkScope(deniedJWTRequest, "data:config.read") {
		t.Fatal("expected false on service_role JWT without required scope")
	}
}

func TestDataControlPlaneHandlerBaseInvalidateCacheUnit(t *testing.T) {
	ctx := context.Background()

	// With nil service
	handlerWithoutServiceControlPlaneHandler := NewControlPlaneHandler(nil, nil, nil)
	handlerWithoutServiceControlPlaneHandler.invalidateCache(ctx)

	// With service
	service := NewService(nil)
	handlerWithServiceControlPlaneHandler := service.controlPlaneHandler
	handlerWithServiceControlPlaneHandler.invalidateCache(ctx)
}
