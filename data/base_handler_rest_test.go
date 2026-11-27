package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/graphql"
	"layr.sh/data/rest"
)

func TestDataRESTMetadataEdgeCasesUnit(t *testing.T) {
	baseHandler := NewService(core.NewTestKernel(nil)).BaseHandler()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/auth/users", nil)
	responseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, "*", baseHandler.buildSelectColumns(request, rest.TableMetadata{}))
	queryParams := &rest.QueryParams{Filters: []rest.FilterOp{{Column: "invalid-name", Op: "eq"}}}
	assert.Error(t, writeExactCountHeaders(request.Context(), responseRecorder, nil, rest.NewQueryBuilder("public", "items"), queryParams, 0))
	baseHandler.SetTableMetadata(rest.TableMetadata{Schema: "public", Table: "items", ForeignKeys: map[string]rest.RelationForeignKey{"parent": {ToSchema: "related", ToTable: "parents"}}})
	before := baseHandler.getCompositeTableCacheVersion(request.Context(), "public", "items", []string{"parent"})
	baseHandler.bumpTableCacheVersion(request.Context(), "related", "parents")
	assert.NotEqual(t, before, baseHandler.getCompositeTableCacheVersion(request.Context(), "public", "items", []string{"parent"}))
}

func TestDataRESTMissingCatalogUnit(t *testing.T) {
	for _, unavailable := range []bool{true, false} {
		t.Run(fmt.Sprint(unavailable), func(t *testing.T) {
			kernel := postgresFaultKernel(t, func(sql string) []pgproto3.BackendMessage {
				if catalogQueryStage(sql) == "schema tables" {
					if unavailable {
						return postgresFailure()
					}
					return postgresResult([]uint32{25})
				}
				return catalogQueryReply(sql)
			})
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/missing", nil)
			responseRecorder := httptest.NewRecorder()
			NewService(kernel).BaseHandler().handleListRecords(responseRecorder, request)
			expected := http.StatusNotFound
			if unavailable {
				expected = http.StatusInternalServerError
			}
			assert.Equal(t, expected, responseRecorder.Code, responseRecorder.Body.String())
		})
	}
}

func TestDataRowDecodingFailuresAndJSONValuesUnit(t *testing.T) {
	baseHandler := NewService(core.NewTestKernel(nil)).BaseHandler()
	for _, rows := range []*mockJSONRows{{rows: [][]any{{}}, valueErr: errors.New("decode")}, {streamErr: errors.New("stream")}} {
		t.Run("case", func(t *testing.T) {
			if _, err := baseHandler.scanRowsToJSONMaps(rows); err == nil {
				t.Fatal("row failure swallowed")
			}
		})
	}
	if queryJSONValue(uint8('i'), 18) != "i" || queryJSONValue(uint8(4), 0) != uint8(4) {
		t.Fatal("internal char decoded incorrectly")
	}
	for _, pair := range []struct {
		value    float64
		expected any
	}{{1, float64(1)}, {math.NaN(), "NaN"}, {math.Inf(1), "Infinity"}, {math.Inf(-1), "-Infinity"}} {
		t.Run("case", func(t *testing.T) {
			if value := queryJSONValue(pair.value, 0); value != pair.expected {
				t.Fatalf("unexpected JSON numeric value: %v", value)
			}
		})
	}
	if _, err := json.Marshal(queryJSONValue(map[string]any{"ids": []any{[16]byte{}, math.Inf(1)}}, 0)); err != nil {
		t.Fatal(err)
	}
	chars := queryJSONValue([]any{uint8('a')}, 1002).([]any)
	if chars[0] != "a" {
		t.Fatal("char array decoded incorrectly")
	}
	for _, code := range []string{"22P02", "22003", "22007", "22008", "23502", "23514", "25006"} {
		t.Run("case", func(t *testing.T) {
			responseRecorder := httptest.NewRecorder()
			baseHandler.writeDBErrorResponse(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil), &pgconn.PgError{Code: code})
			if responseRecorder.Code != http.StatusBadRequest {
				t.Fatal("invalid input reported as server error")
			}
		})
	}
	invalidClaimsCtx := core.WithAuthContext(context.Background(), core.AuthContext{JWT: core.JWTClaims{Claims: map[string]any{"unsupported": func() {}}}})
	if baseHandler.queryCacheKey(httptest.NewRequestWithContext(invalidClaimsCtx, http.MethodGet, "/", nil), "rest:alias", "rest:query") != "" {
		t.Fatal("invalid claims were cacheable")
	}
}

func TestDataBaseHandlerRESTValidationUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	configManager := service.configManager
	baseHandler := service.baseHandler
	baseHandler.SetTableMetadata(rest.TableMetadata{Schema: "public", Table: "users", PrimaryKey: "id", Columns: []string{"id", "name"}})

	t.Run("DisabledREST", func(t *testing.T) {
		config := configManager.Get()
		config.REST.Enabled = false
		configManager.SetMemoryConfig(config)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// restore
		config.REST.Enabled = true
		configManager.SetMemoryConfig(config)
	})

	t.Run("InvalidIdentifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/bad-schema/users", nil)
		request.SetPathValue("schema_name", "bad-schema")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UnexposedSchema", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/private/users", nil)
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

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/secret", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "secret")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		config.REST.ExcludedTables = nil
		configManager.SetMemoryConfig(config)
	})

	t.Run("GetRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users/", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CreateRecordEmptyBody", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users", bytes.NewReader([]byte{}))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CreateRecordEmptyArray", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users", bytes.NewReader([]byte(`[]`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/public/users/", bytes.NewReader([]byte(`{"name":"Alice"}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordEmptyBody", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/public/users/123", bytes.NewReader([]byte{}))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		request.SetPathValue("record_id", "123")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UpdateRecordInvalidJSON", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/public/users/123", bytes.NewReader([]byte(`invalid`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		request.SetPathValue("record_id", "123")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("DeleteRecordMissingID", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/data/public/users/", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleDeleteRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("BuildSelectColumns", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users?select=id,name,email", nil)
		selectedColumns := baseHandler.buildSelectColumns(request, rest.TableMetadata{})
		assert.Equal(t, `"id", "name", "email"`, selectedColumns)

		tableMetadata := rest.TableMetadata{Columns: []string{"id", "title"}}
		defaultRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users", nil)
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
		baseHandler.handleGetRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleDeleteRecord(responseRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/data/public/users/1", nil))
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("CreateRecordInvalidJSONVariations", func(t *testing.T) {
		// Bad array json
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users", bytes.NewReader([]byte(`[bad json`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad object json
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users", bytes.NewReader([]byte(`{bad json`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleCreateRecord(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("PathParsingFallbacks", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users/rec-123", nil)
		s, tbl, id, err := baseHandler.extractSchemaTableAndRecordID(request)
		assert.NoError(t, err)
		assert.Equal(t, "public", s)
		assert.Equal(t, "users", tbl)
		assert.Equal(t, "rec-123", id)

		// Invalid short path
		badRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/invalid", nil)
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
		// getTableCacheVersion coverage
		assert.Equal(t, int64(0), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:users", "bad_num", 0)
		assert.Equal(t, int64(0), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:users", "42", 0)
		assert.Equal(t, int64(42), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))

		// invalidateTableCache
		baseHandler.InvalidateTableCache(context.Background(), "public", "users")
		assert.Equal(t, int64(43), baseHandler.getTableCacheVersion(context.Background(), "public", "users"))

		// with InvalidateOnMutation disabled
		tableConfig := configManager.Get()
		tableConfig.Cache.InvalidateOnMutation = false
		configManager.SetMemoryConfig(tableConfig)
		baseHandler.InvalidateTableCache(context.Background(), "public", "users")

		// with Cache disabled
		tableConfig.Cache.Enabled = false
		configManager.SetMemoryConfig(tableConfig)
		baseHandler.InvalidateTableCache(context.Background(), "public", "users")

		// restore
		tableConfig.Cache.Enabled = true
		tableConfig.Cache.InvalidateOnMutation = true
		configManager.SetMemoryConfig(tableConfig)
	})

	t.Run("ListRecordsInvalidQueryParams", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/users?limit=not_a_number", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("table_name", "users")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("PrepareTableContextInvalidPath", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/invalid", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleListRecords(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("CompositeAndResetTableCacheVersion", func(t *testing.T) {
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
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:users", "1", 0)
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:orders", "2", 0)
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:items", "3", 0)
		compositeVersion := baseHandler.getCompositeTableCacheVersion(context.Background(), "public", "users", relations)
		assert.NotZero(t, compositeVersion)

		// 3. getCompositeTableCacheVersion without relations
		singleVersion := baseHandler.getCompositeTableCacheVersion(context.Background(), "public", "users", nil)
		assert.Equal(t, int64(1), singleVersion)

		// 4. Invalidate rollover guard
		_ = baseHandler.kernel.KVStore().Set(context.Background(), "cache:v:public:rollover", "9000000000000000", 0)
		baseHandler.InvalidateTableCache(context.Background(), "public", "rollover")
		rolloverValue, _ := baseHandler.kernel.KVStore().Get(context.Background(), "cache:v:public:rollover")
		assert.Equal(t, "1", rolloverValue)

		// 5. ResetTableCacheVersion
		baseHandler.ResetTableCacheVersion(context.Background(), "public", "users")
		usersValue, _ := baseHandler.kernel.KVStore().Get(context.Background(), "cache:v:public:users")
		assert.Empty(t, usersValue)
	})

	t.Run("CreateRecordInvalidOnConflict", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/users?on_conflict=bad-id", bytes.NewReader([]byte(`[{"name":"Alice"}]`)))
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
		results, scanErr := baseHandler.scanRowsToJSONMaps(rows)
		assert.NoError(t, scanErr)
		assert.Len(t, results, 1)
		marshaled, err := json.Marshal(results[0])
		assert.NoError(t, err)
		assert.Contains(t, string(marshaled), `"json_col":{"key":"val1"}`)
		assert.Contains(t, string(marshaled), `"jsonb_col":{"key":"val2"}`)
	})
}

func TestDataBaseHandlerRESTRPCValidationUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	configManager := service.configManager
	baseHandler := service.baseHandler

	t.Run("MethodNotAllowed", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/data/public/rpc/test", nil)
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

		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/test", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "test")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("MissingNames", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidIdentifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/bad-fn;drop", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "bad-fn;drop")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("UnexposedSchema", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/private/rpc/my_func", nil)
		request.SetPathValue("schema_name", "private")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{invalid`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidArgumentNamePOST", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{"bad-name":1}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("InvalidArgumentNameGET", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/rpc/my_func?bad-name=1", nil)
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("DatabaseUnavailable", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/public/rpc/my_func", bytes.NewReader([]byte(`{"a":1}`)))
		request.SetPathValue("schema_name", "public")
		request.SetPathValue("function_name", "my_func")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("PathFallback", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/public/rpc/my_func", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleExecuteFunction(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("ParseQueryArgValueTypes", func(t *testing.T) {
		assert.Equal(t, int64(123), parseQueryArgValue("123"))
		assert.Equal(t, json.Number("123.45"), parseQueryArgValue("123.45"))
		assert.Equal(t, true, parseQueryArgValue("true"))
		assert.Equal(t, false, parseQueryArgValue("false"))
		assert.Equal(t, map[string]any{"x": json.Number("1")}, parseQueryArgValue(`{"x":1}`))
		assert.Equal(t, []any{json.Number("1"), json.Number("2")}, parseQueryArgValue(`[1,2]`))
		assert.Equal(t, "hello", parseQueryArgValue("hello"))
	})
}

type mockJSONRows struct {
	fields    []pgconn.FieldDescription
	rows      [][]any
	index     int
	valueErr  error
	streamErr error
}

func (m *mockJSONRows) Close()                                       {}
func (m *mockJSONRows) Err() error                                   { return m.streamErr }
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
func (m *mockJSONRows) Values() ([]any, error) { return m.rows[m.index-1], m.valueErr }
func (m *mockJSONRows) RawValues() [][]byte    { return nil }
func (m *mockJSONRows) Conn() *pgx.Conn        { return nil }

var _ pgx.Rows = (*mockJSONRows)(nil)

func TestDataHandlerWireFailuresUnit(t *testing.T) {
	for _, testCase := range []struct{ operation, stage string }{
		{"list", "rls"}, {"get", "rls"}, {"create", "rls"}, {"update", "rls"}, {"delete", "rls"}, {"graphql", "rls"}, {"rpc", "rls"}, {"rpc", "readonly"},
		{"list", "values"}, {"get", "values"}, {"update", "values"}, {"rpc", "values"}, {"list", "count"}, {"list", "commit"}, {"get", "commit"}, {"list", "marshal"},
	} {
		t.Run(testCase.operation+"/"+testCase.stage, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			kernel := postgresFaultKernel(t, func(sql string) []pgproto3.BackendMessage {
				statement := strings.ToLower(strings.TrimSpace(sql))
				target := false
				switch testCase.stage {
				case "rls":
					target = strings.Contains(statement, "set local role")
				case "readonly":
					target = strings.Contains(statement, "set transaction read only")
				case "commit":
					target = statement == "commit"
				case "count":
					target = strings.Contains(statement, "select count(*)")
				case "values", "marshal":
					target = strings.HasPrefix(statement, "select ") || strings.HasPrefix(statement, "update ")
				}
				if target {
					select {
					case reached <- struct{}{}:
					default:
					}
					if testCase.stage == "values" {
						return postgresResult([]uint32{23}, []string{"invalid-integer"})
					}
					if testCase.stage != "marshal" {
						return postgresFailure()
					}
				}
				if strings.HasPrefix(statement, "select ") || strings.HasPrefix(statement, "update ") {
					return postgresResult([]uint32{23}, []string{"1"})
				}
				return nil
			})
			if testCase.stage == "marshal" {
				connection, err := kernel.DB().Acquire(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				connection.Conn().TypeMap().RegisterType(&pgtype.Type{Name: "int4", OID: 23, Codec: unmarshalableIntCodec{}})
				connection.Release()
			}
			service := NewService(kernel)
			service.SetTableMetadata(rest.TableMetadata{Schema: "public", Table: "items", PrimaryKey: "id", Columns: []string{"id"}})
			service.GraphQLSchema().SetTable(&graphql.TableInfo{Schema: "public", Name: "items", PrimaryKey: "id", Columns: map[string]graphql.ColumnInfo{"id": {Name: "id", DataType: "integer"}}})
			method, path, body := http.MethodGet, "/v1/data/public/items", ""
			switch testCase.operation {
			case "get":
				path += "/1"
			case "create":
				method = http.MethodPost
				body = `{"id":1}`
			case "update":
				method = http.MethodPatch
				path += "/1"
				body = `{"id":2}`
			case "delete":
				method = http.MethodDelete
				path += "/1"
			case "graphql":
				method = http.MethodPost
				path = "/v1/graphql"
				body = `{"query":"{ items(limit:1.5) { id } }"}`
			case "rpc":
				path = "/v1/data/public/rpc/echo"
			}
			if testCase.stage == "count" {
				path += "?count=exact"
			}
			request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
			responseRecorder := httptest.NewRecorder()
			baseHandler := service.BaseHandler()
			switch testCase.operation {
			case "list":
				baseHandler.handleListRecords(responseRecorder, request)
			case "get":
				baseHandler.handleGetRecord(responseRecorder, request)
			case "create":
				baseHandler.handleCreateRecord(responseRecorder, request)
			case "update":
				baseHandler.handleUpdateRecord(responseRecorder, request)
			case "delete":
				baseHandler.handleDeleteRecord(responseRecorder, request)
			case "graphql":
				baseHandler.handleExecuteGraphQL(responseRecorder, request)
			case "rpc":
				baseHandler.handleExecuteFunction(responseRecorder, request)
			}
			if len(reached) == 0 || responseRecorder.Code != http.StatusInternalServerError {
				t.Fatalf("database failure not propagated: reached=%v status=%d body=%s", (len(reached) > 0), responseRecorder.Code, responseRecorder.Body.String())
			}
		})
	}
}
