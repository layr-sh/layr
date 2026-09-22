package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/graphql"
	datakv "layr.sh/data/kv"
)

func TestDataBaseHandlerGraphQLUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	configManager := service.configManager
	baseHandler := service.baseHandler

	t.Run("MethodNotAllowed", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/graphql", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusMethodNotAllowed, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "only supports POST")
	})

	t.Run("DisabledGraphQL", func(t *testing.T) {
		config := configManager.Get()
		config.GraphQL.Enabled = false
		configManager.SetMemoryConfig(config)
		defer func() {
			config.GraphQL.Enabled = true
			configManager.SetMemoryConfig(config)
		}()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"query { test }"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "GraphQL API is disabled")
	})

	t.Run("InvalidJSONPayload", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Invalid JSON request payload")
	})

	t.Run("EmptyQueryString", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"   "}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "cannot be empty")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"bad syntax {"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "syntax error")
	})

	t.Run("MaxDepthExceeded", func(t *testing.T) {
		config := configManager.Get()
		config.GraphQL.MaxDepth = 1
		configManager.SetMemoryConfig(config)
		defer func() {
			config.GraphQL.MaxDepth = 8
			configManager.SetMemoryConfig(config)
		}()

		deepQuery := `query { users { posts { comments { id } } } }`
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"`+deepQuery+`"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusUnprocessableEntity, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "exceeds maximum allowed depth")
	})

	t.Run("ComplexityExceeded", func(t *testing.T) {
		complexFields := ""
		for i := 0; i < 600; i++ {
			complexFields += "id "
		}
		query := `query { users { ` + complexFields + `} }`
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"`+query+`"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusUnprocessableEntity, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "exceeds limit")
	})

	t.Run("CacheHit", func(t *testing.T) {
		query := `query @cache(ttl: 60) { users { id } }`
		authContext := datakv.ExtractAuthContext(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", nil), "")
		userVisibleKey := datakv.BuildGraphQLQueryKey(query, nil, "")
		internalKey := datakv.BuildInternalKey(authContext, userVisibleKey)

		_ = kernel.KVStore().Set(context.Background(), internalKey, `{"data":{"users":[{"id":"1"}]}}`, time.Hour)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"`+query+`"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, "HIT", responseRecorder.Header().Get("X-Layr-Cache"))
		assert.Contains(t, responseRecorder.Body.String(), `"id":"1"`)
	})

	t.Run("DefaultMaxDepthFallback", func(t *testing.T) {
		config := configManager.Get()
		config.GraphQL.MaxDepth = 0
		config.GraphQL.MaxComplexity = 0
		configManager.SetMemoryConfig(config)
		defer func() {
			config.GraphQL.MaxDepth = 8
			config.GraphQL.MaxComplexity = 500
			configManager.SetMemoryConfig(config)
		}()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"query { test { id } }"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("CompilationError", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"mutation { unsupported_op { id } }"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "compilation error")
	})

	t.Run("DatabaseUnavailable", func(t *testing.T) {
		query := `query { test { id } }`
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"`+query+`"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "closed pool")
	})

	t.Run("writeGraphQLDBError", func(t *testing.T) {
		codes := []string{"42501", "23505", "23503", "42P01", "42703", "unknown"}
		expectedStatuses := []int{
			http.StatusForbidden,
			http.StatusConflict,
			http.StatusConflict,
			http.StatusNotFound,
			http.StatusBadRequest,
			http.StatusInternalServerError,
		}

		for index, code := range codes {
			responseRecorder := httptest.NewRecorder()
			if code == "unknown" {
				baseHandler.writeGraphQLDBError(responseRecorder, errors.New("generic db error"))
			} else {
				pgError := &pgconn.PgError{Code: code, Message: "test", Detail: "detail"}
				baseHandler.writeGraphQLDBError(responseRecorder, pgError)
			}
			assert.Equal(t, expectedStatuses[index], responseRecorder.Code, "code %s failed", code)
		}
	})

	t.Run("IntrospectionSchemaAndType", func(t *testing.T) {
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "public",
			Name:   "users",
			Columns: map[string]graphql.ColumnInfo{
				"id": {Name: "id", DataType: "uuid"},
			},
			ForeignKeys: map[string]graphql.RelationInfo{
				"posts": {ForeignTable: "posts"},
			},
		})

		// __schema query
		schemaQuery := `query { __schema { types { name } } }`
		schemaBytes, _ := json.Marshal(ExecuteGraphQLInput{Query: schemaQuery})
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(schemaBytes))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"users"`)

		// __type query found
		typeQuery := `query { __type(name: "users") { name fields { name } } }`
		typeBytes, _ := json.Marshal(ExecuteGraphQLInput{Query: typeQuery})
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(typeBytes))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"users"`)

		// __type query not found
		typeNotFoundQuery := `query { __type(name: "nonexistent") { name } }`
		typeNotFoundBytes, _ := json.Marshal(ExecuteGraphQLInput{Query: typeNotFoundQuery})
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(typeNotFoundBytes))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"__type":null`)

		// Introspection disabled
		config := configManager.Get()
		config.GraphQL.IntrospectionEnabled = false
		configManager.SetMemoryConfig(config)
		defer func() {
			config.GraphQL.IntrospectionEnabled = true
			configManager.SetMemoryConfig(config)
		}()
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(typeBytes))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("handleMutationSideEffects", func(t *testing.T) {
		operationNode := &graphql.OperationNode{
			Type: graphql.MutationOperationType,
			SelectionSet: []graphql.FieldNode{
				{Name: "insert_tenant_a_orders"},
				{Name: "update_users"},
				{Name: "delete_products"},
				{Name: "unsupported_action"},
			},
		}
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "tenant_a",
			Name:   "orders",
		})
		baseHandler.handleMutationSideEffects(context.Background(), operationNode)
	})

	t.Run("MultiOperationSelect", func(t *testing.T) {
		multiQuery := "query OpA { posts { id } }\nquery OpB { posts { title } }"
		multiBytes, _ := json.Marshal(ExecuteGraphQLInput{
			Query:         multiQuery,
			OperationName: "OpB",
		})
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(multiBytes))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "closed pool")
	})

	t.Run("DefaultAndMaxLimitEnforcement", func(t *testing.T) {
		// 1. Fallback when GraphQL DefaultLimit is 0 and REST MaxLimit is 0
		config := configManager.Get()
		config.GraphQL.DefaultLimit = 0
		config.REST.MaxLimit = 0
		configManager.SetMemoryConfig(config)
		defer func() {
			config.GraphQL.DefaultLimit = 100
			config.REST.MaxLimit = 1000
			configManager.SetMemoryConfig(config)
		}()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"query { users { id } }"}`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// 2. Clamped when limit exceeds REST MaxLimit (literal int64)
		config.REST.MaxLimit = 100
		configManager.SetMemoryConfig(config)
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader([]byte(`{"query":"query { users(limit: 500) { id } }"}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// 3. Clamped when limit comes from variable (float64 from json unmarshal)
		varLimitBytes, _ := json.Marshal(ExecuteGraphQLInput{
			Query:     "query($lim: Int) { users(limit: $lim) { id } }",
			Variables: map[string]any{"lim": float64(500)},
		})
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/graphql", bytes.NewReader(varLimitBytes))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleExecuteGraphQL(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("GraphQLTableResolutionAndVersioning", func(t *testing.T) {
		// 1. resolveGraphQLTables(nil)
		assert.Empty(t, baseHandler.resolveGraphQLTables(nil))

		// 2. getGraphQLTableCacheVersion(nil)
		assert.Equal(t, int64(0), baseHandler.getGraphQLTableCacheVersion(context.Background(), nil))

		// 3. Schema prefix matching & relations matching
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "audit",
			Name:   "logs",
		})
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "audit",
			Name:   "user",
		})
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "audit",
			Name:   "items",
		})
		baseHandler.schemaIntrospector.SetTable(&graphql.TableInfo{
			Schema: "audit",
			Name:   "tags",
		})

		operationNode := &graphql.OperationNode{
			SelectionSet: []graphql.FieldNode{
				{
					Name: "audit_logs",
					SelectionSet: []graphql.FieldNode{
						{
							Name:         "users",
							SelectionSet: []graphql.FieldNode{{Name: "id"}},
						},
						{
							Name:         "item",
							SelectionSet: []graphql.FieldNode{{Name: "id"}},
						},
						{
							Name:         "tags",
							SelectionSet: []graphql.FieldNode{{Name: "id"}},
						},
						{
							Name:         "other",
							SelectionSet: []graphql.FieldNode{{Name: "id"}},
						},
						{
							Name:         "leaf_field",
							SelectionSet: nil,
						},
					},
				},
			},
		}

		tables := baseHandler.resolveGraphQLTables(operationNode)
		assert.NotEmpty(t, tables)

		// 4. Single table version
		singleTableVersion := baseHandler.getGraphQLTableCacheVersion(context.Background(), tables[:1])
		assert.Equal(t, int64(0), singleTableVersion)

		// 5. Multi-table composite version
		compositeVersion := baseHandler.getGraphQLTableCacheVersion(context.Background(), tables)
		assert.NotZero(t, compositeVersion)
	})
}
