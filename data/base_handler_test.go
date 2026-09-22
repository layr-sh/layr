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
	"layr.sh/data/rest"
)

func TestDataBaseHandlerInitializationUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := NewBaseHandler(service)
	assert.NotNil(t, baseHandler)
	assert.NotNil(t, baseHandler.RealtimeHub())
	assert.NotNil(t, baseHandler.GraphQLSchema())

	service.SetSaltSecret("test-salt")
	assert.Equal(t, "test-salt", service.saltSecret)

	service.SetTableMetadata(rest.TableMetadata{
		Schema:     "public",
		Table:      "users",
		PrimaryKey: "id",
	})
	assert.Contains(t, service.tables, "public.users")
}

func TestDataBaseHandlerHelperMethodsUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := NewBaseHandler(service)

	t.Run("parsePath", func(t *testing.T) {
		schema, table, recordID, err := baseHandler.parsePath("/v1/data/public/users/123")
		assert.NoError(t, err)
		assert.Equal(t, "public", schema)
		assert.Equal(t, "users", table)
		assert.Equal(t, "123", recordID)

		schema, table, recordID, err = baseHandler.parsePath("/v1/data/public/users")
		assert.NoError(t, err)
		assert.Equal(t, "public", schema)
		assert.Equal(t, "users", table)
		assert.Empty(t, recordID)

		_, _, _, err = baseHandler.parsePath("/v1/data/invalid")
		assert.Error(t, err)
	})

	t.Run("writeError", func(t *testing.T) {
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		core.WriteErrorResponse(responseRecorder, request, http.StatusBadRequest, "bad request")
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("writeDBErrorResponse", func(t *testing.T) {
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
				baseHandler.writeDBErrorResponse(responseRecorder, request, errors.New("generic db error"))
			} else {
				pgError := &pgconn.PgError{Code: code, Message: "test", Detail: "detail"}
				baseHandler.writeDBErrorResponse(responseRecorder, request, pgError)
			}
			assert.Equal(t, expectedStatuses[index], responseRecorder.Code, "code %s failed", code)
		}
	})

	t.Run("isRLSBypassed", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		assert.False(t, baseHandler.isRLSBypassed(request, core.ScopeDataQueryRead))

		jwtRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), core.AuthContext{
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: core.ScopeDataQueryRead,
			},
		}), http.MethodGet, "/test", nil)
		assert.True(t, baseHandler.isRLSBypassed(jwtRequest, core.ScopeDataQueryRead))

		noScopeRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), core.AuthContext{
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: "other:scope",
			},
		}), http.MethodGet, "/test", nil)
		assert.False(t, baseHandler.isRLSBypassed(noScopeRequest, core.ScopeDataQueryRead))

		badKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		badKeyRequest.Header.Set("Authorization", "Bearer invalid_key")
		assert.False(t, baseHandler.isRLSBypassed(badKeyRequest, core.ScopeDataQueryRead))
	})

	t.Run("resolveTransaction", func(t *testing.T) {
		ctx := context.Background()
		assert.Error(t, baseHandler.resolveTransaction(ctx, nil, errors.New("fail")))
		assert.NoError(t, baseHandler.resolveTransaction(ctx, nil, nil))
	})

	t.Run("IntrospectSchemasHelper", func(t *testing.T) {
		ctx := context.Background()
		assert.Error(t, baseHandler.IntrospectSchemas(ctx))
	})
}
