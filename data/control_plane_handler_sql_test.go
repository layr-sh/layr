package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerSQLScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(nil)
	service := NewService(nil)
	service.SetServiceAccountManager(serviceAccountManager)
	controlPlaneHandler := service.controlPlaneHandler

	forbiddenSQLRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/sql", bytes.NewReader([]byte(`{"sql":"SELECT 1"}`)))
	forbiddenSQLRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenSQLResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(forbiddenSQLResponseRecorder, forbiddenSQLRequest)
	if forbiddenSQLResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthorized SQL execution, got %d", forbiddenSQLResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerSQLValidationUnit(t *testing.T) {
	ctx := context.Background()
	service := NewService(nil)
	controlPlaneHandler := service.controlPlaneHandler

	// Method Not Allowed
	invalidMethodRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/sql", nil)
	invalidMethodResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(invalidMethodResponseRecorder, invalidMethodRequest)
	if invalidMethodResponseRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET /sql, got %d", invalidMethodResponseRecorder.Code)
	}

	// Malformed JSON
	malformedJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/sql", bytes.NewReader([]byte("not json")))
	malformedJSONResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(malformedJSONResponseRecorder, malformedJSONRequest)
	if malformedJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", malformedJSONResponseRecorder.Code)
	}

	// Empty SQL query
	emptySQLRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/sql", bytes.NewReader([]byte(`{"sql":""}`)))
	emptySQLResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(emptySQLResponseRecorder, emptySQLRequest)
	if emptySQLResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty SQL query, got %d", emptySQLResponseRecorder.Code)
	}
}
