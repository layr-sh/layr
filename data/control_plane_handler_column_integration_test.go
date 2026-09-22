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

func TestDataControlPlaneHandlerColumnLifecycleIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	_ = service.Start(ctx)
	defer func() { service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// Create base table for column testing
	createTableBody, _ := json.Marshal(CreateTableInput{
		Schema: "public",
		Name:   "column_test_table",
		Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "initial_name", Type: "text", IsNullable: false},
		},
	})
	createTableRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader(createTableBody))
	createTableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(createTableResponseRecorder, createTableRequest)
	if createTableResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create table, got %d", createTableResponseRecorder.Code)
	}

	// 1. Add Column
	addColumnBody, _ := json.Marshal(Column{
		Name:       "bio",
		Type:       "text",
		IsNullable: true,
	})
	addRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/column_test_table/columns", bytes.NewReader(addColumnBody))
	addRequest.SetPathValue("schema_name", "public")
	addRequest.SetPathValue("table_name", "column_test_table")
	addResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateColumn(addResponseRecorder, addRequest)
	if addResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on add column, got %d", addResponseRecorder.Code)
	}

	// Add Column error (duplicate or invalid column type)
	invalidAddRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/column_test_table/columns", bytes.NewReader(addColumnBody))
	invalidAddRequest.SetPathValue("schema_name", "public")
	invalidAddRequest.SetPathValue("table_name", "column_test_table")
	invalidAddResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateColumn(invalidAddResponseRecorder, invalidAddRequest)
	if invalidAddResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on duplicate add column, got %d", invalidAddResponseRecorder.Code)
	}

	// 2. Alter Column
	newName := "biography"
	alterBody, _ := json.Marshal(UpdateColumnInput{
		NewName: &newName,
	})
	alterRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/column_test_table/columns/bio", bytes.NewReader(alterBody))
	alterRequest.SetPathValue("schema_name", "public")
	alterRequest.SetPathValue("table_name", "column_test_table")
	alterRequest.SetPathValue("column_name", "bio")
	alterResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateColumn(alterResponseRecorder, alterRequest)
	if alterResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on alter column, got %d", alterResponseRecorder.Code)
	}

	// Alter Column error (non-existent column)
	invalidAlterRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/data/tables/public/column_test_table/columns/nonexistent", bytes.NewReader(alterBody))
	invalidAlterRequest.SetPathValue("schema_name", "public")
	invalidAlterRequest.SetPathValue("table_name", "column_test_table")
	invalidAlterRequest.SetPathValue("column_name", "nonexistent")
	invalidAlterResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateColumn(invalidAlterResponseRecorder, invalidAlterRequest)
	if invalidAlterResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on alter non-existent column, got %d", invalidAlterResponseRecorder.Code)
	}

	// 3. Drop Column
	dropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/public/column_test_table/columns/biography?cascade=true", nil)
	dropRequest.SetPathValue("schema_name", "public")
	dropRequest.SetPathValue("table_name", "column_test_table")
	dropRequest.SetPathValue("column_name", "biography")
	dropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteColumn(dropResponseRecorder, dropRequest)
	if dropResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on drop column, got %d", dropResponseRecorder.Code)
	}

	// Drop Column error (protected schema)
	invalidDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/core/nodes/columns/nonexistent", nil)
	invalidDropRequest.SetPathValue("schema_name", "core")
	invalidDropRequest.SetPathValue("table_name", "nodes")
	invalidDropRequest.SetPathValue("column_name", "nonexistent")
	invalidDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteColumn(invalidDropResponseRecorder, invalidDropRequest)
	if invalidDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on drop column in protected schema, got %d", invalidDropResponseRecorder.Code)
	}
}
