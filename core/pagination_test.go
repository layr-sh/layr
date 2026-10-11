package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorePaginationParsingUnit(t *testing.T) {
	t.Run("default pagination", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		paginationInput := ParsePagination(request)
		assert.Equal(t, DefaultPaginationLimit, paginationInput.Limit)
		assert.Equal(t, 0, paginationInput.Offset)
	})

	t.Run("custom valid pagination", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test?limit=25&offset=50", nil)
		paginationInput := ParsePagination(request)
		assert.Equal(t, 25, paginationInput.Limit)
		assert.Equal(t, 50, paginationInput.Offset)
	})

	t.Run("limit exceeds max limit clamped", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test?limit=5000&offset=10", nil)
		paginationInput := ParsePagination(request)
		assert.Equal(t, MaxPaginationLimit, paginationInput.Limit)
		assert.Equal(t, 10, paginationInput.Offset)
	})

	t.Run("invalid non-numeric parameters fall back to defaults", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test?limit=abc&offset=-5", nil)
		paginationInput := ParsePagination(request)
		assert.Equal(t, DefaultPaginationLimit, paginationInput.Limit)
		assert.Equal(t, 0, paginationInput.Offset)
	})
}

func TestCoreRoutePaginationOpenAPIUnit(t *testing.T) {
	engine := fuego.NewServer(fuego.WithAddr("localhost:0"))
	router := NewRouter(engine)

	GetRoute[string](router, "/items", func(responseWriter http.ResponseWriter, request *http.Request) {},
		RouteTag("Items"),
		RouteDefaultPagination(),
		RouteQueryParam("search", "Search query"),
		RouteOperationID("items__list"),
		RouteSDKGroupName("items"),
		RouteSDKMethodName("list"),
	)

	openAPISpec := router.OutputOpenAPISpec()
	require.NotNil(t, openAPISpec)

	pathItem := openAPISpec.Paths.Find("/items")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Get)
	require.Len(t, pathItem.Get.Parameters, 3)

	paramNames := make(map[string]string)
	for _, parameter := range pathItem.Get.Parameters {
		paramNames[parameter.Value.Name] = parameter.Value.In
	}

	assert.Equal(t, "query", paramNames["limit"])
	assert.Equal(t, "query", paramNames["offset"])
	assert.Equal(t, "query", paramNames["search"])
}
