package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerTableScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Forbidden ListTables
	forbiddenListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	forbiddenListRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListTables(forbiddenListResponseRecorder, forbiddenListRequest)
	if forbiddenListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenListResponseRecorder.Code)
	}

	// Forbidden CreateTable
	forbiddenCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader([]byte(`{}`)))
	forbiddenCreateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(forbiddenCreateResponseRecorder, forbiddenCreateRequest)
	if forbiddenCreateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenCreateResponseRecorder.Code)
	}

	// Forbidden GetTable
	forbiddenGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/public/users", nil)
	forbiddenGetRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetTable(forbiddenGetResponseRecorder, forbiddenGetRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenGetResponseRecorder.Code)
	}

	// Forbidden DropTable
	forbiddenDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/public/users", nil)
	forbiddenDropRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteTable(forbiddenDropResponseRecorder, forbiddenDropRequest)
	if forbiddenDropResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenDropResponseRecorder.Code)
	}

	// Forbidden TruncateTable
	forbiddenTruncateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/users/truncate", nil)
	forbiddenTruncateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenTruncateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleTruncateTable(forbiddenTruncateResponseRecorder, forbiddenTruncateRequest)
	if forbiddenTruncateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenTruncateResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerTableValidationAndMissingParamsUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Malformed JSON on CreateTable
	malformedCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader([]byte("not json")))
	malformedCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(malformedCreateResponseRecorder, malformedCreateRequest)
	if malformedCreateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedCreateResponseRecorder.Code)
	}

	// Missing parameters on GetTable
	missingParamsGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	missingParamsGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetTable(missingParamsGetResponseRecorder, missingParamsGetRequest)
	if missingParamsGetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingParamsGetResponseRecorder.Code)
	}

	// Missing parameters on DropTable
	missingParamsDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables", nil)
	missingParamsDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteTable(missingParamsDropResponseRecorder, missingParamsDropRequest)
	if missingParamsDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingParamsDropResponseRecorder.Code)
	}

	// Missing parameters on TruncateTable
	missingParamsTruncateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", nil)
	missingParamsTruncateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleTruncateTable(missingParamsTruncateResponseRecorder, missingParamsTruncateRequest)
	if missingParamsTruncateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingParamsTruncateResponseRecorder.Code)
	}

	// CreateTable on protected schema error
	protectedSchemaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader([]byte(`{"schema":"core","name":"test"}`)))
	protectedSchemaResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(protectedSchemaResponseRecorder, protectedSchemaRequest)
	if protectedSchemaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on protected schema, got %d", protectedSchemaResponseRecorder.Code)
	}
}
