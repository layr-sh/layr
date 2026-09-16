package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerColumnScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(nil)
	service := NewService(nil)
	service.SetServiceAccountManager(serviceAccountManager)
	controlPlaneHandler := service.controlPlaneHandler

	// Forbidden AddColumn
	forbiddenAddRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/columns", bytes.NewReader([]byte(`{}`)))
	forbiddenAddRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenAddResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAddColumn(forbiddenAddResponseRecorder, forbiddenAddRequest)
	if forbiddenAddResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenAddResponseRecorder.Code)
	}

	// Forbidden AlterColumn
	forbiddenAlterRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/users/columns/name", bytes.NewReader([]byte(`{}`)))
	forbiddenAlterRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenAlterResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAlterColumn(forbiddenAlterResponseRecorder, forbiddenAlterRequest)
	if forbiddenAlterResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenAlterResponseRecorder.Code)
	}

	// Forbidden DropColumn
	forbiddenDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/users/columns/name", nil)
	forbiddenDropRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDropColumn(forbiddenDropResponseRecorder, forbiddenDropRequest)
	if forbiddenDropResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbiddenDropResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerColumnValidationAndMissingParamsUnit(t *testing.T) {
	ctx := context.Background()
	service := NewService(nil)
	controlPlaneHandler := service.controlPlaneHandler

	// Missing parameters on AddColumn
	missingAddRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables", nil)
	missingAddResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAddColumn(missingAddResponseRecorder, missingAddRequest)
	if missingAddResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing params, got %d", missingAddResponseRecorder.Code)
	}

	// Malformed JSON on AddColumn
	malformedAddRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/users/columns", bytes.NewReader([]byte("not json")))
	malformedAddRequest.SetPathValue("schema_name", "public")
	malformedAddRequest.SetPathValue("table_name", "users")
	malformedAddResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAddColumn(malformedAddResponseRecorder, malformedAddRequest)
	if malformedAddResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedAddResponseRecorder.Code)
	}

	// Missing parameters on AlterColumn
	missingAlterRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/users/columns", nil)
	missingAlterRequest.SetPathValue("schema_name", "public")
	missingAlterRequest.SetPathValue("table_name", "users")
	missingAlterResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAlterColumn(missingAlterResponseRecorder, missingAlterRequest)
	if missingAlterResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing column name, got %d", missingAlterResponseRecorder.Code)
	}

	// Malformed JSON on AlterColumn
	malformedAlterRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/users/columns/name", bytes.NewReader([]byte("not json")))
	malformedAlterRequest.SetPathValue("schema_name", "public")
	malformedAlterRequest.SetPathValue("table_name", "users")
	malformedAlterRequest.SetPathValue("column_name", "name")
	malformedAlterResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleAlterColumn(malformedAlterResponseRecorder, malformedAlterRequest)
	if malformedAlterResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedAlterResponseRecorder.Code)
	}

	// Missing parameters on DropColumn
	missingDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/users/columns", nil)
	missingDropRequest.SetPathValue("schema_name", "public")
	missingDropRequest.SetPathValue("table_name", "users")
	missingDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDropColumn(missingDropResponseRecorder, missingDropRequest)
	if missingDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing column name, got %d", missingDropResponseRecorder.Code)
	}
}
