package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
)

func TestCoreRouterWrapperMethodsUnit(t *testing.T) {
	server := NewServer(nil, nil)
	router := server.Router()

	// Test nil engine router creation
	if nilRouter := NewRouter(nil); nilRouter.Engine() != nil {
		t.Fatal("expected nil Engine() from NewRouter(nil)")
	}

	if router.Engine() == nil {
		t.Fatal("expected non-nil Engine()")
	}
	if router.Mux() == nil {
		t.Fatal("expected non-nil Mux()")
	}

	// Register test routes covering all router methods
	GetRoute[string](router, "/test-get", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-get"))
	}, RouteTag("Test"), RouteSummary("Test GET"), RouteDescription("Test GET Desc"), RouteOperationID("testGet"))
	PostRoute[string, map[string]string](router, "/test-post", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusCreated)
		_, _ = responseWriter.Write([]byte("ok-post"))
	}, RouteTag("Test"), RouteSummary("Test POST"), RouteDescription("Test POST Desc"), RouteOperationID("testPost"))
	PutRoute[string, map[string]string](router, "/test-put", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-put"))
	}, RouteTag("Test"), RouteSummary("Test PUT"), RouteDescription("Test PUT Desc"), RouteOperationID("testPut"))
	DeleteRoute[string](router, "/test-delete", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusNoContent)
	}, RouteTag("Test"), RouteSummary("Test DELETE"), RouteDescription("Test DELETE Desc"), RouteOperationID("testDelete"))
	HeadRoute[string](router, "/test-head", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}, RouteTag("Test"), RouteSummary("Test HEAD"), RouteDescription("Test HEAD Desc"), RouteOperationID("testHead"))
	PatchRoute[string, map[string]string](router, "/test-patch", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-patch"))
	}, RouteTag("Test"), RouteSummary("Test PATCH"), RouteDescription("Test PATCH Desc"), RouteOperationID("testPatch"))

	// Test invoking registered routes
	getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test-get", nil)
	getRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-get, got %d", getRecorder.Code)
	}

	headRequest := httptest.NewRequestWithContext(context.Background(), http.MethodHead, "/test-head", nil)
	headRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(headRecorder, headRequest)
	if headRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-head, got %d", headRecorder.Code)
	}

	postRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test-post", strings.NewReader(`{"key":"value"}`))
	postRequest.Header.Set("Content-Type", "application/json")
	postRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(postRecorder, postRequest)
	if postRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 from test-post, got %d", postRecorder.Code)
	}

	putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/test-put", strings.NewReader(`{"key":"value"}`))
	putRequest.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(putRecorder, putRequest)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-put, got %d", putRecorder.Code)
	}

	deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/test-delete", nil)
	deleteRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from test-delete, got %d", deleteRecorder.Code)
	}

	patchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/test-patch", strings.NewReader(`{"key":"value"}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-patch, got %d", patchRecorder.Code)
	}
}

func TestCoreRouterHelpersAndSanitizeOpenAPIUnit(t *testing.T) {
	engine := fuego.NewServer(fuego.WithAddr("localhost:0"))
	router := NewRouter(engine)

	PostRoute[string, map[string]string](router, "/test-helpers", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusCreated)
	},
		RouteDefaultStatusCode(http.StatusCreated),
		RouteResponseHeader("X-Test-Header", "test header description"),
		RouteBinaryResponse(http.StatusOK, "binary payload", "application/octet-stream"),
		RouteBinaryResponse(http.StatusOK, "default binary"),
		RouteNoContentResponse("no content response"),
		RouteResponseModel(http.StatusOK, "model response", map[string]string{}),
		RouteRequestBodyModel(map[string]string{}, "application/json"),
		RouteSDKGroupName("data", "cache"),
		RouteSDKMethodName("testHelper"),
	)

	// Test calling on raw BaseRoute with nil Responses and Extensions to cover initialization
	emptyBaseRoute := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteBinaryResponse(http.StatusOK, "raw binary")(emptyBaseRoute)
	emptyBaseRoute2 := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteNoContentResponse("raw no content")(emptyBaseRoute2)
	emptyBaseRoute3 := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteSDKGroupName("data", "cache")(emptyBaseRoute3)
	RouteSDKMethodName("customMethod")(emptyBaseRoute3)
	// Call again with non-nil Extensions
	RouteSDKGroupName("data", "store")(emptyBaseRoute3)
	RouteSDKMethodName("anotherMethod")(emptyBaseRoute3)

	emptyBaseRoute4 := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteSDKMethodName("standaloneMethod")(emptyBaseRoute4)
	RouteNoRequestBody()(emptyBaseRoute4)

	// Router OutputOpenAPISpec test on empty engine
	emptyRouter := &Router{}
	if emptyRouter.OutputOpenAPISpec() != nil {
		t.Fatalf("expected nil from OutputOpenAPISpec on empty router")
	}
}

func TestCoreRouterTypedMethodsUnit(t *testing.T) {
	engine := fuego.NewServer(fuego.WithAddr("localhost:0"))
	router := NewRouter(engine)

	type DummyResponse struct {
		Message string `json:"message"`
	}
	type DummyRequest struct {
		Name string `json:"name"`
	}

	dummyHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}

	GetRoute[DummyResponse](router, "/test/typed/get", dummyHandler, RouteTag("Test"), RouteSummary("Get Typed"))
	HeadRoute[DummyResponse](router, "/test/typed/head", dummyHandler, RouteTag("Test"), RouteSummary("Head Typed"))
	PostRoute[DummyResponse, DummyRequest](router, "/test/typed/post", dummyHandler, RouteTag("Test"), RouteSummary("Post Typed"))
	PutRoute[DummyResponse, DummyRequest](router, "/test/typed/put", dummyHandler, RouteTag("Test"), RouteSummary("Put Typed"))
	DeleteRoute[DummyResponse](router, "/test/typed/delete", dummyHandler, RouteTag("Test"), RouteSummary("Delete Typed"))
	PatchRoute[DummyResponse, DummyRequest](router, "/test/typed/patch", dummyHandler, RouteTag("Test"), RouteSummary("Patch Typed"))

	// Test HTTP dispatching
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		request := httptest.NewRequestWithContext(context.Background(), method, "/test/typed/"+strings.ToLower(method), strings.NewReader(`{"name":"layr"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.Mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d", method, recorder.Code)
		}
	}
}
