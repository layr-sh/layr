package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/rest"
)

func TestDataBaseHandlerRESTValidationUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)

	t.Run("DisabledREST", func(t *testing.T) {
		config := configManager.Get()
		config.REST.Enabled = false
		configManager.SetMemoryConfig(config)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// restore
		config.REST.Enabled = true
		configManager.SetMemoryConfig(config)
	})

	t.Run("InvalidIdentifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/bad-schema/users", nil)
		request.SetPathValue("schema_name", "bad-schema")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UnexposedSchema", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/private/users", nil)
		request.SetPathValue("schema_name", "private")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("ExcludedTable", func(t *testing.T) {
		config := configManager.Get()
		config.REST.ExcludedTables = []string{"secret"}
		configManager.SetMemoryConfig(config)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/secret", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "secret")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		config.REST.ExcludedTables = nil
		configManager.SetMemoryConfig(config)
	})

	t.Run("GetRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users/", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CreateRecordEmptyBody", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users", bytes.NewReader([]byte{}))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CreateRecordEmptyArray", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users", bytes.NewReader([]byte(`[]`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/public/users/", bytes.NewReader([]byte(`{"name":"Alice"}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordEmptyBody", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/public/users/123", bytes.NewReader([]byte{}))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		request.SetPathValue("record_id", "123")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordInvalidJSON", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/public/users/123", bytes.NewReader([]byte(`invalid`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		request.SetPathValue("record_id", "123")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("DeleteRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/data/public/users/", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("BuildSelectColumns", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users?select=id,name,email", nil)
		selectedColumns := baseHandler.buildSelectColumns(request, rest.TableMetadata{})
		assert.Equal(t, `"id", "name", "email"`, selectedColumns)

		tableMetadata := rest.TableMetadata{Columns: []string{"id", "title"}}
		defaultRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users", nil)
		defaultColumns := baseHandler.buildSelectColumns(defaultRequest, tableMetadata)
		assert.Equal(t, `"id", "title"`, defaultColumns)
	})

	t.Run("DisabledRESTAllOperations", func(t *testing.T) {
		config := configManager.Get()
		config.REST.Enabled = false
		configManager.SetMemoryConfig(config)
		defer func() {
			config.REST.Enabled = true
			configManager.SetMemoryConfig(config)
		}()

		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleDeleteRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("CreateRecordInvalidJSONVariations", func(t *testing.T) {
		// Bad array json
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users", bytes.NewReader([]byte(`[bad json`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad object json
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users", bytes.NewReader([]byte(`{bad json`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("PathParsingFallbacks", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users/rec-123", nil)
		s, tbl, id, err := baseHandler.extractSchemaTableAndRecordID(request)
		assert.NoError(t, err)
		assert.Equal(t, "public", s)
		assert.Equal(t, "users", tbl)
		assert.Equal(t, "rec-123", id)

		// Invalid short path
		badRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/invalid", nil)
		_, _, _, err = baseHandler.extractSchemaTableAndRecordID(badRequest)
		assert.Error(t, err)
	})

	t.Run("ReturnMinimalHeader", func(t *testing.T) {
		minimalRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test", nil)
		minimalRequest.Header.Set("Prefer", "return=minimal")
		assert.True(t, isReturnMinimal(minimalRequest))

		representationRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test", nil)
		representationRequest.Header.Set("Prefer", "return=representation")
		assert.False(t, isReturnMinimal(representationRequest))
	})

	t.Run("InvalidateTableCache", func(t *testing.T) {
		inMemoryKVDriver := newInMemoryKVDriver()
		inMemoryKVStore := core.NewKVStoreFromDriver(inMemoryKVDriver)
		baseHandler.SetKVStore(inMemoryKVStore)

		// getTableCacheVersion coverage
		assert.Equal(t, int64(0), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))
		inMemoryKVDriver.storage["cache:v:public:users"] = "bad_num"
		assert.Equal(t, int64(0), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))
		inMemoryKVDriver.storage["cache:v:public:users"] = "42"
		assert.Equal(t, int64(42), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))

		// invalidateTableCache
		baseHandler.InvalidateTableCache(context.Background(), "public", "users")
		assert.Equal(t, int64(43), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))

		// with InvalidateOnMutation disabled
		tableConfig := configManager.Get()
		tableConfig.Cache.InvalidateOnMutation = false
		configManager.SetMemoryConfig(tableConfig)
		baseHandler.invalidateTableCache(context.Background(), "public", "users")

		// with Cache disabled
		tableConfig.Cache.Enabled = false
		configManager.SetMemoryConfig(tableConfig)
		baseHandler.invalidateTableCache(context.Background(), "public", "users")

		// restore
		tableConfig.Cache.Enabled = true
		tableConfig.Cache.InvalidateOnMutation = true
		configManager.SetMemoryConfig(tableConfig)

		// nil kvStore
		baseHandler.SetKVStore(nil)
		assert.Equal(t, int64(0), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))
		baseHandler.invalidateTableCache(context.Background(), "public", "users")
		baseHandler.SetKVStore(inMemoryKVStore)
	})

	t.Run("ListRecordsInvalidQueryParams", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/users?limit=not_a_number", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("PrepareTableContextInvalidPath", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/invalid", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CompositeAndResetTableCacheVersion", func(t *testing.T) {
		inMemoryKVDriver := newInMemoryKVDriver()
		inMemoryKVStore := core.NewKVStoreFromDriver(inMemoryKVDriver)
		baseHandler.SetKVStore(inMemoryKVStore)

		// 1. collectEmbeddedRelations with children
		embedded := []rest.EmbeddedField{
			{
				Relation: "orders",
				Children: []rest.EmbeddedField{
					{Relation: "items"},
				},
			},
		}
		relations := collectEmbeddedRelations(embedded)
		assert.Equal(t, []string{"orders", "items"}, relations)

		// 2. getCompositeTableCacheVersion with relations
		inMemoryKVDriver.storage["cache:v:public:users"] = "1"
		inMemoryKVDriver.storage["cache:v:public:orders"] = "2"
		inMemoryKVDriver.storage["cache:v:public:items"] = "3"
		compositeVersion := baseHandler.getCompositeTableCacheVersion(context.Background(), "public", "users", relations)
		assert.NotZero(t, compositeVersion)

		// 3. getCompositeTableCacheVersion without relations
		singleVersion := baseHandler.getCompositeTableCacheVersion(context.Background(), "public", "users", nil)
		assert.Equal(t, int64(1), singleVersion)

		// 4. Invalidate rollover guard
		inMemoryKVDriver.storage["cache:v:public:rollover"] = "9000000000000000"
		baseHandler.InvalidateTableCache(context.Background(), "public", "rollover")
		assert.Equal(t, "1", inMemoryKVDriver.storage["cache:v:public:rollover"])

		// 5. ResetTableCacheVersion
		baseHandler.ResetTableCacheVersion(context.Background(), "public", "users")
		assert.NotContains(t, inMemoryKVDriver.storage, "cache:v:public:users")

		// 6. ResetTableCacheVersion with nil kvStore
		baseHandler.SetKVStore(nil)
		baseHandler.ResetTableCacheVersion(context.Background(), "public", "users")
		baseHandler.SetKVStore(inMemoryKVStore)
	})

	t.Run("CreateRecordInvalidOnConflict", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/users?on_conflict=bad-id", bytes.NewReader([]byte(`[{"name":"Alice"}]`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("ScanRowsToJSONMapsRawJSONMessage", func(t *testing.T) {
		rows := &mockJSONRows{
			fields: []pgconn.FieldDescription{
				{Name: "json_col", DataTypeOID: 114},
				{Name: "jsonb_col", DataTypeOID: 3802},
			},
			rows: [][]any{
				{[]byte(`{"key":"val1"}`), []byte(`{"key":"val2"}`)},
			},
		}
		results := baseHandler.scanRowsToJSONMaps(rows)
		assert.Len(t, results, 1)
		marshaled, err := json.Marshal(results[0])
		assert.NoError(t, err)
		assert.Contains(t, string(marshaled), `"json_col":{"key":"val1"}`)
		assert.Contains(t, string(marshaled), `"jsonb_col":{"key":"val2"}`)
	})
}

func TestDataBaseHandlerRESTRPCValidationUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)

	t.Run("MethodNotAllowed", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/data/public/rpc/test", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "test")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusMethodNotAllowed, responseRecorder.Code)
	})

	t.Run("DisabledREST", func(t *testing.T) {
		config := configManager.Get()
		config.REST.Enabled = false
		configManager.SetMemoryConfig(config)
		defer func() {
			config.REST.Enabled = true
			configManager.SetMemoryConfig(config)
		}()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/test", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "test")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("MissingNames", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidIdentifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/bad-fn;drop", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "bad-fn;drop")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UnexposedSchema", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/private/rpc/my_func", nil)
		request.SetPathValue("schema_name", "private")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{invalid`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidArgumentNamePOST", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{"bad-name":1}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidArgumentNameGET", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/rpc/my_func?bad-name=1", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("DatabaseNil", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{"a":1}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("PathFallback", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/public/rpc/my_func", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("ParseQueryArgValueTypes", func(t *testing.T) {
		assert.Equal(t, int64(123), parseQueryArgValue("123"))
		assert.Equal(t, 123.45, parseQueryArgValue("123.45"))
		assert.Equal(t, true, parseQueryArgValue("true"))
		assert.Equal(t, false, parseQueryArgValue("false"))
		assert.Equal(t, map[string]any{"x": float64(1)}, parseQueryArgValue(`{"x":1}`))
		assert.Equal(t, []any{float64(1), float64(2)}, parseQueryArgValue(`[1,2]`))
		assert.Equal(t, "hello", parseQueryArgValue("hello"))
	})
}

type mockJSONRows struct {
	fields []pgconn.FieldDescription
	rows   [][]any
	index  int
}

func (m *mockJSONRows) Close()                                       {}
func (m *mockJSONRows) Err() error                                   { return nil }
func (m *mockJSONRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockJSONRows) FieldDescriptions() []pgconn.FieldDescription { return m.fields }
func (m *mockJSONRows) Next() bool {
	if m.index < len(m.rows) {
		m.index++
		return true
	}
	return false
}
func (m *mockJSONRows) Scan(dest ...any) error { return nil }
func (m *mockJSONRows) Values() ([]any, error) { return m.rows[m.index-1], nil }
func (m *mockJSONRows) RawValues() [][]byte    { return nil }
func (m *mockJSONRows) Conn() *pgx.Conn        { return nil }

var _ pgx.Rows = (*mockJSONRows)(nil)
