package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerPolicyLifecycleIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	_ = service.Start(ctx)
	defer func() { service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// Create table for policy testing
	createTableBody, err := json.Marshal(CreateTableInput{
		Schema: "public",
		Name:   "policy_test_table",
		Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "user_id", Type: "uuid", IsNullable: false},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal create table request: %v", err)
	}
	createTableRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader(createTableBody))
	createTableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(createTableResponseRecorder, createTableRequest)
	if createTableResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create table, got %d", createTableResponseRecorder.Code)
	}

	// 1. Toggle RLS - ENABLE
	enableRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls?action=ENABLE", nil)
	enableRequest.SetPathValue("schema_name", "public")
	enableRequest.SetPathValue("table_name", "policy_test_table")
	enableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(enableResponseRecorder, enableRequest)
	if enableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on enable RLS, got %d", enableResponseRecorder.Code)
	}

	// 2. Toggle RLS - FORCE true
	forceRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls?action=FORCE&force=true", nil)
	forceRequest.SetPathValue("schema_name", "public")
	forceRequest.SetPathValue("table_name", "policy_test_table")
	forceResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(forceResponseRecorder, forceRequest)
	if forceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on force RLS, got %d", forceResponseRecorder.Code)
	}

	// 3. Toggle RLS - FORCE false
	unforceByFlagRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls?action=FORCE&force=false", nil)
	unforceByFlagRequest.SetPathValue("schema_name", "public")
	unforceByFlagRequest.SetPathValue("table_name", "policy_test_table")
	unforceByFlagResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(unforceByFlagResponseRecorder, unforceByFlagRequest)
	if unforceByFlagResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on unforce by flag RLS, got %d", unforceByFlagResponseRecorder.Code)
	}

	// 4. Toggle RLS - UNFORCE
	unforceRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls?action=UNFORCE", nil)
	unforceRequest.SetPathValue("schema_name", "public")
	unforceRequest.SetPathValue("table_name", "policy_test_table")
	unforceResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(unforceResponseRecorder, unforceRequest)
	if unforceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on UNFORCE RLS, got %d", unforceResponseRecorder.Code)
	}

	// 5. Toggle RLS - Path suffix /enable, /disable, /force
	pathEnableRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls/enable", nil)
	pathEnableRequest.SetPathValue("schema_name", "public")
	pathEnableRequest.SetPathValue("table_name", "policy_test_table")
	pathEnableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(pathEnableResponseRecorder, pathEnableRequest)
	if pathEnableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on path /enable, got %d", pathEnableResponseRecorder.Code)
	}

	pathForceRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls/force", nil)
	pathForceRequest.SetPathValue("schema_name", "public")
	pathForceRequest.SetPathValue("table_name", "policy_test_table")
	pathForceResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(pathForceResponseRecorder, pathForceRequest)
	if pathForceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on path /force, got %d", pathForceResponseRecorder.Code)
	}

	pathDisableRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls/disable", nil)
	pathDisableRequest.SetPathValue("schema_name", "public")
	pathDisableRequest.SetPathValue("table_name", "policy_test_table")
	pathDisableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(pathDisableResponseRecorder, pathDisableRequest)
	if pathDisableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on path /disable, got %d", pathDisableResponseRecorder.Code)
	}

	// Toggle RLS error on non-existent table
	nonExistentRLSRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/nonexistent_table/rls?action=ENABLE", nil)
	nonExistentRLSRequest.SetPathValue("schema_name", "public")
	nonExistentRLSRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentRLSResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(nonExistentRLSResponseRecorder, nonExistentRLSRequest)
	if nonExistentRLSResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent table RLS enable, got %d", nonExistentRLSResponseRecorder.Code)
	}

	nonExistentRLSDisableRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/nonexistent_table/rls?action=DISABLE", nil)
	nonExistentRLSDisableRequest.SetPathValue("schema_name", "public")
	nonExistentRLSDisableRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentRLSDisableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(nonExistentRLSDisableResponseRecorder, nonExistentRLSDisableRequest)
	if nonExistentRLSDisableResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent table RLS disable, got %d", nonExistentRLSDisableResponseRecorder.Code)
	}

	nonExistentRLSForceRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/nonexistent_table/rls?action=FORCE", nil)
	nonExistentRLSForceRequest.SetPathValue("schema_name", "public")
	nonExistentRLSForceRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentRLSForceResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(nonExistentRLSForceResponseRecorder, nonExistentRLSForceRequest)
	if nonExistentRLSForceResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent table RLS force, got %d", nonExistentRLSForceResponseRecorder.Code)
	}

	nonExistentRLSUnforceRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/nonexistent_table/rls?action=UNFORCE", nil)
	nonExistentRLSUnforceRequest.SetPathValue("schema_name", "public")
	nonExistentRLSUnforceRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentRLSUnforceResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(nonExistentRLSUnforceResponseRecorder, nonExistentRLSUnforceRequest)
	if nonExistentRLSUnforceResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent table RLS unforce, got %d", nonExistentRLSUnforceResponseRecorder.Code)
	}

	// 6. Create Policy
	createPolicyBody, err := json.Marshal(CreatePolicyInput{
		Name:            "allow_all_authenticated",
		Command:         "SELECT",
		UsingExpression: "true",
	})
	if err != nil {
		t.Fatalf("failed to marshal create policy request: %v", err)
	}
	createPolicyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/policy_test_table/policies", bytes.NewReader(createPolicyBody))
	createPolicyRequest.SetPathValue("schema_name", "public")
	createPolicyRequest.SetPathValue("table_name", "policy_test_table")
	createPolicyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreatePolicy(createPolicyResponseRecorder, createPolicyRequest)
	if createPolicyResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create policy, got %d, body: %s", createPolicyResponseRecorder.Code, createPolicyResponseRecorder.Body.String())
	}

	// Create Policy error (invalid expression)
	invalidPolicyBody, err := json.Marshal(CreatePolicyInput{
		Name:            "invalid_policy",
		Command:         "SELECT",
		UsingExpression: "syntax error here",
	})
	if err != nil {
		t.Fatalf("failed to marshal invalid policy request: %v", err)
	}
	invalidPolicyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/policy_test_table/policies", bytes.NewReader(invalidPolicyBody))
	invalidPolicyRequest.SetPathValue("schema_name", "public")
	invalidPolicyRequest.SetPathValue("table_name", "policy_test_table")
	invalidPolicyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreatePolicy(invalidPolicyResponseRecorder, invalidPolicyRequest)
	if invalidPolicyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid policy syntax, got %d", invalidPolicyResponseRecorder.Code)
	}

	// 7. List Policies
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/public/policy_test_table/policies", nil)
	listRequest.SetPathValue("schema_name", "public")
	listRequest.SetPathValue("table_name", "policy_test_table")
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListPolicies(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list policies, got %d", listResponseRecorder.Code)
	}

	// List Policies error (protected schema)
	invalidListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/core/policy_test_table/policies", nil)
	invalidListRequest.SetPathValue("schema_name", "core")
	invalidListRequest.SetPathValue("table_name", "policy_test_table")
	invalidListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListPolicies(invalidListResponseRecorder, invalidListRequest)
	if invalidListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on protected schema table policies list, got %d", invalidListResponseRecorder.Code)
	}

	// 8. Drop Policy
	dropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/public/policy_test_table/policies/allow_all_authenticated", nil)
	dropRequest.SetPathValue("schema_name", "public")
	dropRequest.SetPathValue("table_name", "policy_test_table")
	dropRequest.SetPathValue("policy_name", "allow_all_authenticated")
	dropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeletePolicy(dropResponseRecorder, dropRequest)
	if dropResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on drop policy, got %d", dropResponseRecorder.Code)
	}

	// Drop Policy error (protected schema)
	invalidDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/core/policy_test_table/policies/allow_all_authenticated", nil)
	invalidDropRequest.SetPathValue("schema_name", "core")
	invalidDropRequest.SetPathValue("table_name", "policy_test_table")
	invalidDropRequest.SetPathValue("policy_name", "allow_all_authenticated")
	invalidDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeletePolicy(invalidDropResponseRecorder, invalidDropRequest)
	if invalidDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on drop protected schema policy, got %d", invalidDropResponseRecorder.Code)
	}

	// 9. Additional RLS toggle branches
	// Mode: ENABLE
	modeRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls", bytes.NewReader([]byte(`{"mode":"ENABLE"}`)))
	modeRequest.SetPathValue("schema_name", "public")
	modeRequest.SetPathValue("table_name", "policy_test_table")
	modeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(modeResponseRecorder, modeRequest)
	if modeResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on toggle RLS with mode ENABLE, got %d", modeResponseRecorder.Code)
	}

	// Action: UNFORCE
	unforceBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls", bytes.NewReader([]byte(`{"action":"UNFORCE"}`)))
	unforceBodyRequest.SetPathValue("schema_name", "public")
	unforceBodyRequest.SetPathValue("table_name", "policy_test_table")
	unforceBodyResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(unforceBodyResponseRecorder, unforceBodyRequest)
	if unforceBodyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on toggle RLS UNFORCE, got %d", unforceBodyResponseRecorder.Code)
	}

	// Action: FORCE with ?force=false
	forceFalseRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls?force=false", bytes.NewReader([]byte(`{"action":"FORCE"}`)))
	forceFalseRequest.SetPathValue("schema_name", "public")
	forceFalseRequest.SetPathValue("table_name", "policy_test_table")
	forceFalseResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(forceFalseResponseRecorder, forceFalseRequest)
	if forceFalseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on toggle RLS FORCE false, got %d", forceFalseResponseRecorder.Code)
	}

	// Unknown action
	unknownRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/policy_test_table/rls", bytes.NewReader([]byte(`{"action":"UNKNOWN"}`)))
	unknownRequest.SetPathValue("schema_name", "public")
	unknownRequest.SetPathValue("table_name", "policy_test_table")
	unknownResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(unknownResponseRecorder, unknownRequest)
	if unknownResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on toggle RLS unknown action, got %d", unknownResponseRecorder.Code)
	}
}
