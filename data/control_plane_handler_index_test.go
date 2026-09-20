package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerIndexScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(nil)
	service := NewService(nil)
	service.SetServiceAccountManager(serviceAccountManager)
	controlPlaneHandler := service.controlPlaneHandler

	// Forbidden ListIndexes
	forbiddenListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/public/users/indexes", nil)
	forbiddenListRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListIndexes(forbiddenListResponseRecorder, forbiddenListRequest)
	if forbiddenListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenListResponseRecorder.Code)
	}

	// Forbidden CreateIndex
	forbiddenCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/indexes", bytes.NewReader([]byte(`{}`)))
	forbiddenCreateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateIndex(forbiddenCreateResponseRecorder, forbiddenCreateRequest)
	if forbiddenCreateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenCreateResponseRecorder.Code)
	}

	// Forbidden DropIndex
	forbiddenDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/users/indexes/idx_users_name", nil)
	forbiddenDropRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteIndex(forbiddenDropResponseRecorder, forbiddenDropRequest)
	if forbiddenDropResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenDropResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerIndexValidationAndMissingParamsUnit(t *testing.T) {
	ctx := context.Background()
	service := NewService(nil)
	controlPlaneHandler := service.controlPlaneHandler

	// Missing parameters on ListIndexes
	missingListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables", nil)
	missingListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListIndexes(missingListResponseRecorder, missingListRequest)
	if missingListResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingListResponseRecorder.Code)
	}

	// Missing parameters on CreateIndex
	missingCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables", nil)
	missingCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateIndex(missingCreateResponseRecorder, missingCreateRequest)
	if missingCreateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingCreateResponseRecorder.Code)
	}

	// Malformed JSON on CreateIndex
	malformedCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/indexes", bytes.NewReader([]byte("not json")))
	malformedCreateRequest.SetPathValue("schema_name", "public")
	malformedCreateRequest.SetPathValue("table_name", "users")
	malformedCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateIndex(malformedCreateResponseRecorder, malformedCreateRequest)
	if malformedCreateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedCreateResponseRecorder.Code)
	}

	// Missing parameters on DropIndex
	missingDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables", nil)
	missingDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteIndex(missingDropResponseRecorder, missingDropRequest)
	if missingDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingDropResponseRecorder.Code)
	}
}
