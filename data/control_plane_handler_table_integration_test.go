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

func TestDataControlPlaneHandlerTableLifecycleIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()

	service := NewService(db)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// 1. List Tables
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListTables(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list tables, got %d", listResponseRecorder.Code)
	}

	// List Tables error (canceled context)
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		errorListRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/_/data/tables", nil)
		errorListResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListTables(errorListResponseRecorder, errorListRequest)
		if errorListResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled context, got %d", errorListResponseRecorder.Code)
		}
	}

	// 2. Create Table
	defaultValue := "true"
	createTableBody, _ := json.Marshal(CreateTableInput{
		Schema: "public",
		Name:   "blog_posts",
		Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "title", Type: "varchar(200)", IsNullable: false},
			{Name: "is_published", Type: "bool", IsNullable: false, DefaultValue: &defaultValue},
		},
	})
	createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", bytes.NewReader(createTableBody))
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateTable(createResponseRecorder, createRequest)
	if createResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create table, got %d, body: %s", createResponseRecorder.Code, createResponseRecorder.Body.String())
	}

	// 3. Get Table details
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/public/blog_posts", nil)
	getRequest.SetPathValue("schema_name", "public")
	getRequest.SetPathValue("table_name", "blog_posts")
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetTable(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on get table, got %d", getResponseRecorder.Code)
	}

	var table Table
	if err := json.NewDecoder(getResponseRecorder.Body).Decode(&table); err != nil {
		t.Fatalf("failed to decode table: %v", err)
	}
	if table.Name != "blog_posts" {
		t.Fatalf("expected blog_posts, got %s", table.Name)
	}

	// Get non-existent table (404)
	nonExistentGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables/public/nonexistent_table", nil)
	nonExistentGetRequest.SetPathValue("schema_name", "public")
	nonExistentGetRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetTable(nonExistentGetResponseRecorder, nonExistentGetRequest)
	if nonExistentGetResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent table, got %d", nonExistentGetResponseRecorder.Code)
	}

	// 4. Truncate Table
	truncateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/blog_posts/truncate?cascade=true", nil)
	truncateRequest.SetPathValue("schema_name", "public")
	truncateRequest.SetPathValue("table_name", "blog_posts")
	truncateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleTruncateTable(truncateResponseRecorder, truncateRequest)
	if truncateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on truncate table, got %d", truncateResponseRecorder.Code)
	}

	// Truncate non-existent table error
	nonExistentTruncateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables/public/nonexistent_table/truncate", nil)
	nonExistentTruncateRequest.SetPathValue("schema_name", "public")
	nonExistentTruncateRequest.SetPathValue("table_name", "nonexistent_table")
	nonExistentTruncateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleTruncateTable(nonExistentTruncateResponseRecorder, nonExistentTruncateRequest)
	if nonExistentTruncateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on truncate non-existent table, got %d", nonExistentTruncateResponseRecorder.Code)
	}

	// 5. Drop Table
	dropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/public/blog_posts?cascade=true", nil)
	dropRequest.SetPathValue("schema_name", "public")
	dropRequest.SetPathValue("table_name", "blog_posts")
	dropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteTable(dropResponseRecorder, dropRequest)
	if dropResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on drop table, got %d", dropResponseRecorder.Code)
	}

	// Drop Table error (protected schema)
	protectedDropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/data/tables/core/nodes", nil)
	protectedDropRequest.SetPathValue("schema_name", "core")
	protectedDropRequest.SetPathValue("table_name", "nodes")
	protectedDropResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteTable(protectedDropResponseRecorder, protectedDropRequest)
	if protectedDropResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on drop protected table, got %d", protectedDropResponseRecorder.Code)
	}
}
