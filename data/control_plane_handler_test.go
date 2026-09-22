package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerBaseConstructorAndSettersUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := NewControlPlaneHandler(service)

	if controlPlaneHandler == nil {
		t.Fatal("expected non-nil controlPlaneHandler")
	}
}

func TestDataControlPlaneHandlerBaseHelpersUnit(t *testing.T) {
	ctx := context.Background()

	// WriteJSONResponse
	jsonResponseRecorder := httptest.NewRecorder()
	core.WriteJSONResponse(jsonResponseRecorder, http.StatusOK, map[string]string{"status": "ok"})
	if jsonResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", jsonResponseRecorder.Code)
	}

	// writeError
	errorResponseRecorder := httptest.NewRecorder()
	errorRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/test", nil)
	core.WriteErrorResponse(errorResponseRecorder, errorRequest, http.StatusBadRequest, "bad request")
	if errorResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", errorResponseRecorder.Code)
	}

	// writeForbidden
	forbiddenResponseRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/test", nil)
	core.WriteErrorResponse(forbiddenResponseRecorder, forbiddenRequest, http.StatusForbidden, "Insufficient scope permissions for this operation")
	if forbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerBasePathExtractorsUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Path Value extractor test
	pathValueRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/myschema/mytable/columns/mycolumn", nil)
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
	fallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/fallback_schema/fallback_table/columns/fallback_column", nil)
	schema, table = controlPlaneHandler.extractSchemaAndTable(fallbackRequest)
	if schema != "fallback_schema" || table != "fallback_table" {
		t.Fatalf("expected fallback_schema/fallback_table, got %s/%s", schema, table)
	}
	if column := controlPlaneHandler.extractColumnName(fallbackRequest); column != "fallback_column" {
		t.Fatalf("expected fallback_column, got %s", column)
	}

	// URL path with indexes
	indexFallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/fallback_schema/fallback_table/indexes/fallback_index", nil)
	if index := controlPlaneHandler.extractIndexName(indexFallbackRequest); index != "fallback_index" {
		t.Fatalf("expected fallback_index, got %s", index)
	}

	// URL path with policies
	policyFallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/fallback_schema/fallback_table/policies/fallback_policy", nil)
	if policy := controlPlaneHandler.extractPolicyName(policyFallbackRequest); policy != "fallback_policy" {
		t.Fatalf("expected fallback_policy, got %s", policy)
	}

	// Edge cases: empty URL path
	emptyPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data", nil)
	emptySchema, emptyTable := controlPlaneHandler.extractSchemaAndTable(emptyPathRequest)
	if emptySchema != "" || emptyTable != "" {
		t.Fatalf("expected empty schema/table, got %s/%s", emptySchema, emptyTable)
	}
	if column := controlPlaneHandler.extractColumnName(emptyPathRequest); column != "" {
		t.Fatalf("expected empty column for empty path, got %s", column)
	}
	if index := controlPlaneHandler.extractIndexName(emptyPathRequest); index != "" {
		t.Fatalf("expected empty index for empty path, got %s", index)
	}
	if policy := controlPlaneHandler.extractPolicyName(emptyPathRequest); policy != "" {
		t.Fatalf("expected empty policy for empty path, got %s", policy)
	}

	// Non-matching path segments
	nonMatchingRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/public/users", nil)
	if index := controlPlaneHandler.extractIndexName(nonMatchingRequest); index != "" {
		t.Fatalf("expected empty index for non-matching path, got %s", index)
	}
	if policy := controlPlaneHandler.extractPolicyName(nonMatchingRequest); policy != "" {
		t.Fatalf("expected empty policy for non-matching path, got %s", policy)
	}
}

func TestDataControlPlaneHandlerBaseRequireScopeUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	serviceAccountManager := controlPlaneHandler.kernel.ServiceAccountManager()

	allowedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	allowedResponseRecorder := httptest.NewRecorder()
	if !serviceAccountManager.RequireScope(allowedResponseRecorder, allowedRequest, core.ScopeDataConfigRead) {
		t.Fatal("expected true on empty service account key")
	}

	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_key")
	invalidResponseRecorder := httptest.NewRecorder()
	if serviceAccountManager.RequireScope(invalidResponseRecorder, invalidKeyRequest, core.ScopeDataConfigRead) {
		t.Fatal("expected false on invalid service account key")
	}

	allowedJWTRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: core.ScopeDataConfigRead,
		},
	}), http.MethodGet, "/v1/_/data/config", nil)
	allowedJWTResponseRecorder := httptest.NewRecorder()
	if !serviceAccountManager.RequireScope(allowedJWTResponseRecorder, allowedJWTRequest, core.ScopeDataConfigRead) {
		t.Fatal("expected true on service_role JWT with required scope")
	}

	deniedJWTRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Role:  "service_role",
			Scope: "other:scope",
		},
	}), http.MethodGet, "/v1/_/data/config", nil)
	deniedJWTResponseRecorder := httptest.NewRecorder()
	if serviceAccountManager.RequireScope(deniedJWTResponseRecorder, deniedJWTRequest, core.ScopeDataConfigRead) {
		t.Fatal("expected false on service_role JWT without required scope")
	}
}

func TestDataControlPlaneHandlerBaseInvalidateCacheUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	controlPlaneHandler.InvalidateCache(ctx, InvalidateCacheInput{All: true})
	controlPlaneHandler.InvalidateTableCache(ctx, "public", "users")
}
