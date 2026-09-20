package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataControlPlaneHandlerSQLExecutionIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	defer cleanup()

	ctx := context.Background()
	service := NewService(db)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// 1. Execute SELECT using "sql" payload field
	sqlPayload := `{"sql":"SELECT 42 AS answer, 'hello' AS greeting"}`
	sqlRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/sql", bytes.NewReader([]byte(sqlPayload)))
	sqlResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(sqlResponseRecorder, sqlRequest)
	if sqlResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on SQL execution, got %d", sqlResponseRecorder.Code)
	}

	var executeSQLResponse ExecuteSQLResponse
	if err := json.NewDecoder(sqlResponseRecorder.Body).Decode(&executeSQLResponse); err != nil {
		t.Fatalf("failed to decode SQL execution response: %v", err)
	}
	if len(executeSQLResponse.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(executeSQLResponse.Rows))
	}

	// 2. Execute SELECT using "query" payload field
	queryPayload := `{"query":"SELECT 100 AS number"}`
	queryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/sql", bytes.NewReader([]byte(queryPayload)))
	queryResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(queryResponseRecorder, queryRequest)
	if queryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on query execution, got %d", queryResponseRecorder.Code)
	}

	// 3. Syntax error in SQL
	syntaxErrorPayload := `{"sql":"SELECT FROM WHERE INVALID"}`
	syntaxErrorRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/sql", bytes.NewReader([]byte(syntaxErrorPayload)))
	syntaxErrorResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleExecuteSQL(syntaxErrorResponseRecorder, syntaxErrorRequest)
	if syntaxErrorResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on SQL syntax error, got %d", syntaxErrorResponseRecorder.Code)
	}
}
