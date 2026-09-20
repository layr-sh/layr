package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"layr.sh/core"
)

func TestDataServiceSchemaAndQueryFlowE2E(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cryptoKeyManager, keyErr := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if keyErr != nil {
		t.Fatalf("failed to create key manager: %v", keyErr)
	}

	dataService := NewService(db)
	mockKVStore := newInMemoryKVStore()
	dataService.SetKVStore(mockKVStore)

	if startErr := dataService.Start(ctx); startErr != nil {
		t.Fatalf("failed to start data service: %v", startErr)
	}
	defer func() { _ = dataService.Stop() }()

	coreServer := core.NewServer(db, cryptoKeyManager)
	dataService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	publicServeMux := coreServer.BaseRouter().Mux()
	controlPlaneServeMux := coreServer.ControlPlaneRouter().Mux()

	// 1. User Journey: Check initial data config
	configRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/config", nil)
	configResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(configResponseRecorder, configRequest)
	if configResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected data config 200, got %d", configResponseRecorder.Code)
	}

	// 2. User Journey: Create table via Control Plane API
	createTablePayload, marshalErr := json.Marshal(CreateTableInput{
		Schema: "public",
		Name:   "books",
		Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "title", Type: "text", IsNullable: false},
			{Name: "author", Type: "text", IsNullable: false},
			{Name: "price", Type: "numeric", IsNullable: false},
		},
	})
	if marshalErr != nil {
		t.Fatalf("failed to marshal create table payload: %v", marshalErr)
	}
	createTableRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables", bytes.NewReader(createTablePayload))
	createTableResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(createTableResponseRecorder, createTableRequest)
	if createTableResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create table, got %d: %s", createTableResponseRecorder.Code, createTableResponseRecorder.Body.String())
	}

	// 3. User Journey: Verify table exists via GET /api/v1/_/data/tables/public/books
	getTableRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/tables/public/books", nil)
	getTableResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(getTableResponseRecorder, getTableRequest)
	if getTableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on get table, got %d", getTableResponseRecorder.Code)
	}

	// Refresh GraphQL introspected schemas
	_ = dataService.IntrospectSchemas(ctx)

	// 4. User Journey: Insert record via REST API
	insertPayload, insertMarshalErr := json.Marshal(map[string]any{
		"title":  "Designing Data-Intensive Applications",
		"author": "Martin Kleppmann",
		"price":  45.50,
	})
	if insertMarshalErr != nil {
		t.Fatalf("failed to marshal insert payload: %v", insertMarshalErr)
	}
	insertRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/books", bytes.NewReader(insertPayload))
	insertRequest.Header.Set("Content-Type", "application/json")
	insertRequest.Header.Set("Prefer", "return=representation")
	insertResponseRecorder := httptest.NewRecorder()
	publicServeMux.ServeHTTP(insertResponseRecorder, insertRequest)
	if insertResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on REST insert, got %d: %s", insertResponseRecorder.Code, insertResponseRecorder.Body.String())
	}

	var insertedBook map[string]any
	if decodeErr := json.NewDecoder(insertResponseRecorder.Body).Decode(&insertedBook); decodeErr != nil {
		t.Fatalf("failed to decode inserted book: %v", decodeErr)
	}
	bookID, ok := insertedBook["id"].(string)
	if !ok || bookID == "" {
		t.Fatalf("expected valid UUIDv7 book id: %+v", insertedBook)
	}

	// 5. User Journey: Query via GraphQL
	graphqlReader := bytes.NewReader([]byte(`{"query":"query { books { id title author } }"}`))
	graphqlRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", graphqlReader)
	graphqlRequest.Header.Set("Content-Type", "application/json")
	graphqlResponseRecorder := httptest.NewRecorder()
	publicServeMux.ServeHTTP(graphqlResponseRecorder, graphqlRequest)
	if graphqlResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on GraphQL query, got %d: %s", graphqlResponseRecorder.Code, graphqlResponseRecorder.Body.String())
	}

	// 6. User Journey: Invalidate Cache
	invalidateReader := bytes.NewReader([]byte(`{"schema":"public","table":"books"}`))
	invalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/invalidate", invalidateReader)
	invalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(invalidateResponseRecorder, invalidateRequest)
	if invalidateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on cache invalidate, got %d", invalidateResponseRecorder.Code)
	}

	// 7. User Journey: Enable RLS on books table
	rlsReader := bytes.NewReader([]byte(`{"action":"enable"}`))
	rlsRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/_/data/tables/public/books/rls", rlsReader)
	rlsResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(rlsResponseRecorder, rlsRequest)
	if rlsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on enable RLS, got %d: %s", rlsResponseRecorder.Code, rlsResponseRecorder.Body.String())
	}

	// 8. User Journey: Execute SQL via Control Plane
	sqlReader := bytes.NewReader([]byte(`{"sql":"SELECT count(*) FROM public.books;"}`))
	sqlRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/sql", sqlReader)
	sqlResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(sqlResponseRecorder, sqlRequest)
	if sqlResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on execute SQL, got %d: %s", sqlResponseRecorder.Code, sqlResponseRecorder.Body.String())
	}

	// 9. Edge Case: Malformed SQL returns 400
	badSQLReader := bytes.NewReader([]byte(`{"sql":"SELECT FROM SYNTAX ERROR;"}`))
	badSQLRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/sql", badSQLReader)
	badSQLResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(badSQLResponseRecorder, badSQLRequest)
	if badSQLResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on syntax error SQL, got %d", badSQLResponseRecorder.Code)
	}

	// 10. User Journey: Drop Table
	dropRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/data/tables/public/books", nil)
	dropResponseRecorder := httptest.NewRecorder()
	controlPlaneServeMux.ServeHTTP(dropResponseRecorder, dropRequest)
	if dropResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on drop table, got %d", dropResponseRecorder.Code)
	}
}
