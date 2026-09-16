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

func TestDataControlPlaneHandlerIndexLifecycleIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	defer cleanup()

	ctx := context.Background()
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()
	service := NewService(db)
	service.SetEventBus(eventBus)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// Create table for index testing
	createTableBody, err := json.Marshal(CreateTableRequest{
		Schema: "public",
		Name:   "index_test_table",
		Columns: []ColumnDefinition{
			{Name: "id", IsPrimaryKey: true},
			{Name: "email", Type: "varchar(200)", IsNullable: false},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal create table request: %v", err)
	}
	createTableRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables", bytes.NewReader(createTableBody))
	createTableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateTable(createTableResponseRecorder, createTableRequest)
	if createTableResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create table, got %d", createTableResponseRecorder.Code)
	}

	// 1. Create Index
	createIndexBody, err := json.Marshal(CreateIndexRequest{
		IndexName: "idx_index_test_email",
		Columns:   []string{"email"},
		Type:      "btree",
	})
	if err != nil {
		t.Fatalf("failed to marshal create index request: %v", err)
	}
	createIndexRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/index_test_table/indexes", bytes.NewReader(createIndexBody))
	createIndexRequest.SetPathValue("schema_name", "public")
	createIndexRequest.SetPathValue("table_name", "index_test_table")
	createIndexResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateIndex(createIndexResponseRecorder, createIndexRequest)
	if createIndexResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create index, got %d", createIndexResponseRecorder.Code)
	}

	// Create Index error (invalid column)
	invalidIndexBody, err := json.Marshal(CreateIndexRequest{
		IndexName: "idx_invalid",
		Columns:   []string{"nonexistent_column"},
	})
	if err != nil {
		t.Fatalf("failed to marshal invalid index request: %v", err)
	}
	invalidIndexRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public/index_test_table/indexes", bytes.NewReader(invalidIndexBody))
	invalidIndexRequest.SetPathValue("schema_name", "public")
	invalidIndexRequest.SetPathValue("table_name", "index_test_table")
	invalidIndexResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateIndex(invalidIndexResponseRecorder, invalidIndexRequest)
	if invalidIndexResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid column, got %d", invalidIndexResponseRecorder.Code)
	}

	// 2. List Indexes
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/public/index_test_table/indexes", nil)
	listRequest.SetPathValue("schema_name", "public")
	listRequest.SetPathValue("table_name", "index_test_table")
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListIndexes(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list indexes, got %d", listResponseRecorder.Code)
	}

	// List Indexes error (protected schema)
	invalidListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/core/index_test_table/indexes", nil)
	invalidListRequest.SetPathValue("schema_name", "core")
	invalidListRequest.SetPathValue("table_name", "index_test_table")
	invalidListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListIndexes(invalidListResponseRecorder, invalidListRequest)
	if invalidListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on protected schema table, got %d", invalidListResponseRecorder.Code)
	}

	// 3. Drop Index
	dropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/index_test_table/indexes/idx_index_test_email", nil)
	dropRequest.SetPathValue("schema_name", "public")
	dropRequest.SetPathValue("table_name", "index_test_table")
	dropRequest.SetPathValue("index_name", "idx_index_test_email")
	dropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDropIndex(dropResponseRecorder, dropRequest)
	if dropResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on drop index, got %d", dropResponseRecorder.Code)
	}

	// Drop Index error (protected schema)
	invalidDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/core/index_test_table/indexes/idx_index_test_email", nil)
	invalidDropRequest.SetPathValue("schema_name", "core")
	invalidDropRequest.SetPathValue("table_name", "index_test_table")
	invalidDropRequest.SetPathValue("index_name", "idx_index_test_email")
	invalidDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDropIndex(invalidDropResponseRecorder, invalidDropRequest)
	if invalidDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on drop protected schema index, got %d", invalidDropResponseRecorder.Code)
	}
}
