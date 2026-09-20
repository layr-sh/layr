package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerPolicyScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(nil)
	service := NewService(nil)
	service.SetServiceAccountManager(serviceAccountManager)
	controlPlaneHandler := service.controlPlaneHandler

	// Forbidden ListPolicies
	forbiddenListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/public/users/policies", nil)
	forbiddenListRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListPolicies(forbiddenListResponseRecorder, forbiddenListRequest)
	if forbiddenListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenListResponseRecorder.Code)
	}

	// Forbidden CreatePolicy
	forbiddenCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/policies", bytes.NewReader([]byte(`{}`)))
	forbiddenCreateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreatePolicy(forbiddenCreateResponseRecorder, forbiddenCreateRequest)
	if forbiddenCreateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenCreateResponseRecorder.Code)
	}

	// Forbidden DropPolicy
	forbiddenDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/users/policies/policy_name", nil)
	forbiddenDropRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeletePolicy(forbiddenDropResponseRecorder, forbiddenDropRequest)
	if forbiddenDropResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenDropResponseRecorder.Code)
	}

	// Forbidden ToggleRLS
	forbiddenToggleRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/users/rls", nil)
	forbiddenToggleRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenToggleResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(forbiddenToggleResponseRecorder, forbiddenToggleRequest)
	if forbiddenToggleResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenToggleResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerPolicyValidationAndMissingParamsUnit(t *testing.T) {
	ctx := context.Background()
	service := NewService(nil)
	controlPlaneHandler := service.controlPlaneHandler

	// Missing parameters on ListPolicies
	missingListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables", nil)
	missingListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListPolicies(missingListResponseRecorder, missingListRequest)
	if missingListResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingListResponseRecorder.Code)
	}

	// Missing parameters on CreatePolicy
	missingCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables", nil)
	missingCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreatePolicy(missingCreateResponseRecorder, missingCreateRequest)
	if missingCreateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingCreateResponseRecorder.Code)
	}

	// Malformed JSON on CreatePolicy
	malformedCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/policies", bytes.NewReader([]byte("not json")))
	malformedCreateRequest.SetPathValue("schema_name", "public")
	malformedCreateRequest.SetPathValue("table_name", "users")
	malformedCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreatePolicy(malformedCreateResponseRecorder, malformedCreateRequest)
	if malformedCreateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedCreateResponseRecorder.Code)
	}

	// Missing parameters on DropPolicy
	missingDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables", nil)
	missingDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeletePolicy(missingDropResponseRecorder, missingDropRequest)
	if missingDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingDropResponseRecorder.Code)
	}

	// Missing parameters on ToggleRLS
	missingToggleRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables", nil)
	missingToggleResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(missingToggleResponseRecorder, missingToggleRequest)
	if missingToggleResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingToggleResponseRecorder.Code)
	}

	// Unknown RLS action on ToggleRLS
	unknownActionRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/users/rls?action=UNKNOWN", nil)
	unknownActionRequest.SetPathValue("schema_name", "public")
	unknownActionRequest.SetPathValue("table_name", "users")
	unknownActionResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleToggleRLS(unknownActionResponseRecorder, unknownActionRequest)
	if unknownActionResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown RLS action, got %d", unknownActionResponseRecorder.Code)
	}
}
