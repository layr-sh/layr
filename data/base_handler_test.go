package data

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/realtime"
	"layr.sh/data/rest"
)

func TestDataBaseHandlerInitializationUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)
	assert.NotNil(t, baseHandler)
	assert.NotNil(t, baseHandler.RealtimeHub())
	assert.NotNil(t, baseHandler.GraphQLSchema())

	aliasBaseHandler := NewHandler(nil, configManager)
	assert.NotNil(t, aliasBaseHandler)

	inMemoryKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(inMemoryKVStore)
	assert.Equal(t, inMemoryKVStore, baseHandler.kvStore)

	eventBus := core.NewEventBus(nil, nil)
	defer eventBus.Close()
	baseHandler.SetEventBus(eventBus)
	assert.Equal(t, eventBus, baseHandler.eventBus)

	newHub := realtime.NewHub(nil)
	baseHandler.SetRealtimeHub(newHub)
	assert.Equal(t, newHub, baseHandler.realtimeHub)

	baseHandler.SetSaltSecret("test-salt")
	assert.Equal(t, "test-salt", baseHandler.saltSecret)

	serviceAccountManager := core.NewServiceAccountManager(nil)
	baseHandler.SetServiceAccountManager(serviceAccountManager)
	assert.Equal(t, serviceAccountManager, baseHandler.serviceAccountManager)

	baseHandler.SetTableMetadata(rest.TableMetadata{
		Schema:     "public",
		Table:      "users",
		PrimaryKey: "id",
	})
	assert.Contains(t, baseHandler.tables, "public.users")
}

func TestDataBaseHandlerHelperMethodsUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)

	t.Run("parsePath", func(t *testing.T) {
		schema, table, recordID, err := baseHandler.parsePath("/api/v1/data/public/users/123")
		assert.NoError(t, err)
		assert.Equal(t, "public", schema)
		assert.Equal(t, "users", table)
		assert.Equal(t, "123", recordID)

		schema, table, recordID, err = baseHandler.parsePath("/api/v1/data/public/users")
		assert.NoError(t, err)
		assert.Equal(t, "public", schema)
		assert.Equal(t, "users", table)
		assert.Empty(t, recordID)

		_, _, _, err = baseHandler.parsePath("/api/v1/data/invalid")
		assert.Error(t, err)
	})

	t.Run("writeJSON", func(t *testing.T) {
		responseRecorder := httptest.NewRecorder()
		baseHandler.writeJSON(responseRecorder, http.StatusOK, map[string]string{"status": "ok"})
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"status":"ok"`)

		createdResponseRecorder := httptest.NewRecorder()
		baseHandler.writeJSON(createdResponseRecorder, http.StatusCreated, map[string]string{"status": "created"})
		assert.Equal(t, http.StatusCreated, createdResponseRecorder.Code)
	})

	t.Run("writeError", func(t *testing.T) {
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		core.WriteErrorResponse(responseRecorder, request, http.StatusBadRequest, "bad request")
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("writeDBError", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)

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
				baseHandler.writeDBError(responseRecorder, request, errors.New("generic db error"))
			} else {
				pgError := &pgconn.PgError{Code: code, Message: "test", Detail: "detail"}
				baseHandler.writeDBError(responseRecorder, request, pgError)
			}
			assert.Equal(t, expectedStatuses[index], responseRecorder.Code, "code %s failed", code)
		}
	})

	t.Run("isRLSBypassed", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		assert.False(t, baseHandler.isRLSBypassed(request, "data:query.read"))

		jwtRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), core.AuthContext{
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: "data:query.read",
			},
		}), http.MethodGet, "/test", nil)
		assert.True(t, baseHandler.isRLSBypassed(jwtRequest, "data:query.read"))

		noScopeRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), core.AuthContext{
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: "other:scope",
			},
		}), http.MethodGet, "/test", nil)
		assert.False(t, baseHandler.isRLSBypassed(noScopeRequest, "data:query.read"))

		baseHandler.SetServiceAccountManager(nil)
		assert.False(t, baseHandler.isRLSBypassed(request, "data:query.read"))
	})

	t.Run("resolveTransaction", func(t *testing.T) {
		ctx := context.Background()
		assert.Error(t, baseHandler.resolveTransaction(ctx, nil, errors.New("fail")))
		assert.NoError(t, baseHandler.resolveTransaction(ctx, nil, nil))
	})

	t.Run("IntrospectSchemasHelper", func(t *testing.T) {
		ctx := context.Background()
		assert.NoError(t, baseHandler.IntrospectSchemas(ctx))

		emptyBaseHandler := &BaseHandler{}
		assert.NoError(t, emptyBaseHandler.IntrospectSchemas(ctx))
	})

	t.Run("SettersNilSafety", func(t *testing.T) {
		emptyBaseHandler := &BaseHandler{tables: make(map[string]rest.TableMetadata)}
		emptyBaseHandler.SetKVStore(nil)
		emptyBaseHandler.SetRealtimeHub(nil)
		assert.Nil(t, emptyBaseHandler.RealtimeHub())
		assert.Nil(t, emptyBaseHandler.GraphQLSchema())
	})
}
