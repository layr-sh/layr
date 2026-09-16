package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"layr.sh/core"
)

func TestDataBaseHandlerGraphQLIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	service := NewService(db)
	inMemoryKVStore := newInMemoryKVStore()
	service.SetKVStore(inMemoryKVStore)
	serviceAccountManager := core.NewServiceAccountManager(db)
	service.SetServiceAccountManager(serviceAccountManager)
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()
	service.SetEventBus(eventBus)

	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	// 1. Create a test table
	createTableRequest := CreateTableRequest{
		Name: "posts",
		Columns: []ColumnDefinition{
			{Name: "id", Type: "uuid", IsPrimaryKey: true},
			{Name: "title", Type: "text", IsNullable: false},
			{Name: "body", Type: "text"},
		},
	}
	tableJSONBytes, _ := json.Marshal(createTableRequest)
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public", bytes.NewReader(tableJSONBytes))
	request.SetPathValue("schema_name", "public")
	responseRecorder := httptest.NewRecorder()
	service.GetControlPlaneHandler().HandleCreateTable(responseRecorder, request)
	assert.Equal(t, http.StatusCreated, responseRecorder.Code)

	// Introspect schemas
	_ = service.IntrospectSchemas(ctx)

	// 2. Insert a row via SQL
	_, err := db.Exec(ctx, `INSERT INTO public.posts (title, body) VALUES ('First Post', 'Hello World');`)
	assert.NoError(t, err)

	baseHandler := service.BaseHandler()

	_ = inMemoryKVStore.Delete(ctx, "cache:schema:catalog")
	canceledSchemaCtx, schemaCancel := context.WithCancel(ctx)
	schemaCancel()
	assert.Error(t, baseHandler.IntrospectSchemas(canceledSchemaCtx))
	_ = service.IntrospectSchemas(ctx)

	// 3. Query via GraphQL
	graphQLQuery := `query { posts { id title body } }`
	graphQLQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"`+graphQLQuery+`"}`)))
	graphQLQueryRequest.Header.Set("Content-Type", "application/json")
	graphQLQueryResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(graphQLQueryResponseRecorder, graphQLQueryRequest)

	assert.Equal(t, http.StatusOK, graphQLQueryResponseRecorder.Code)
	assert.Contains(t, graphQLQueryResponseRecorder.Body.String(), "First Post")

	// 4. Query with Cache TTL
	cachedQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"`+graphQLQuery+`"}`)))
	cachedQueryRequest.Header.Set("Content-Type", "application/json")
	cachedQueryRequest.Header.Set("X-Layr-Cache-TTL", "60")
	cachedQueryRequest.Header.Set("X-Layr-Cache-Key-Suffix", "test-suffix")
	cachedQueryResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(cachedQueryResponseRecorder, cachedQueryRequest)

	assert.Equal(t, http.StatusOK, cachedQueryResponseRecorder.Code)
	assert.Equal(t, "MISS", cachedQueryResponseRecorder.Header().Get("X-Layr-Cache"))

	// Query again -> HIT
	hitQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"`+graphQLQuery+`"}`)))
	hitQueryRequest.Header.Set("Content-Type", "application/json")
	hitQueryRequest.Header.Set("X-Layr-Cache-TTL", "60")
	hitQueryRequest.Header.Set("X-Layr-Cache-Key-Suffix", "test-suffix")
	hitQueryResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(hitQueryResponseRecorder, hitQueryRequest)
	assert.Equal(t, http.StatusOK, hitQueryResponseRecorder.Code)
	assert.Equal(t, "HIT", hitQueryResponseRecorder.Header().Get("X-Layr-Cache"))

	// Caching skipped when MaxCachedQueries exceeded
	appConfig := service.GetConfigManager().Get()
	appConfig.Cache.MaxCachedQueries = 2
	service.GetConfigManager().SetMemoryConfig(appConfig)
	inMemoryKVStore.storage["cache:query_count"] = "5"
	capQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"query { posts { id } }"}`)))
	capQueryRequest.Header.Set("Content-Type", "application/json")
	capQueryRequest.Header.Set("X-Layr-Cache-TTL", "60")
	capQueryRequest.Header.Set("X-Layr-Cache-Key-Suffix", "cap-suffix")
	capQueryResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(capQueryResponseRecorder, capQueryRequest)
	assert.Equal(t, http.StatusOK, capQueryResponseRecorder.Code)
	appConfig.Cache.MaxCachedQueries = 10000
	service.GetConfigManager().SetMemoryConfig(appConfig)

	// 5. GraphQL Mutation
	mutationQuery := `mutation { insert_posts(objects: [{title: "Second Post", body: "Second Body"}]) { id title } }`
	mutationPayloadBytes, _ := json.Marshal(GraphQLRequest{Query: mutationQuery})
	mutationRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader(mutationPayloadBytes))
	mutationRequest.Header.Set("Content-Type", "application/json")
	mutationResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(mutationResponseRecorder, mutationRequest)
	assert.Equal(t, http.StatusOK, mutationResponseRecorder.Code)
	assert.Contains(t, mutationResponseRecorder.Body.String(), "Second Post")

	// 6. DB execution error
	_, _ = db.Exec(ctx, `CREATE TABLE public.temp_gql (id uuid primary key default uuidv7(), name text);`)
	_ = service.IntrospectSchemas(ctx)
	_, _ = db.Exec(ctx, `DROP TABLE public.temp_gql;`)
	dropQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"query { temp_gql { id } }"}`)))
	dropQueryRequest.Header.Set("Content-Type", "application/json")
	dropQueryResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(dropQueryResponseRecorder, dropQueryRequest)
	assert.Equal(t, http.StatusNotFound, dropQueryResponseRecorder.Code)

	// 7. Canceled context on Begin
	canceledGraphQLCtx, cancel := context.WithCancel(ctx)
	cancel()
	canceledGraphQLRequest := httptest.NewRequestWithContext(canceledGraphQLCtx, http.MethodPost, "/api/v1/graphql", bytes.NewReader([]byte(`{"query":"query { posts { id } }"}`)))
	canceledGraphQLRequest.Header.Set("Content-Type", "application/json")
	canceledGraphQLResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGraphQL(canceledGraphQLResponseRecorder, canceledGraphQLRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledGraphQLResponseRecorder.Code)
}
