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
	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})
	router := server.BaseRouter()

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
	}, RouteTag("Test"), RouteSummary("Test GET"), RouteDescription("Test GET Desc"), RouteOperationID("test_get"))
	PostRoute[string, map[string]string](router, "/test-post", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusCreated)
		_, _ = responseWriter.Write([]byte("ok-post"))
	}, RouteTag("Test"), RouteSummary("Test POST"), RouteDescription("Test POST Desc"), RouteOperationID("test_post"))
	PutRoute[string, map[string]string](router, "/test-put", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-put"))
	}, RouteTag("Test"), RouteSummary("Test PUT"), RouteDescription("Test PUT Desc"), RouteOperationID("test_put"))
	DeleteRoute[string](router, "/test-delete", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusNoContent)
	}, RouteTag("Test"), RouteSummary("Test DELETE"), RouteDescription("Test DELETE Desc"), RouteOperationID("test_delete"))
	HeadRoute[string](router, "/test-head", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}, RouteTag("Test"), RouteSummary("Test HEAD"), RouteDescription("Test HEAD Desc"), RouteOperationID("test_head"))
	PatchRoute[string, map[string]string](router, "/test-patch", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-patch"))
	}, RouteTag("Test"), RouteSummary("Test PATCH"), RouteDescription("Test PATCH Desc"), RouteOperationID("test_patch"))

	// Test invoking registered routes
	getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test-get", nil)
	getResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-get, got %d", getResponseRecorder.Code)
	}

	headRequest := httptest.NewRequestWithContext(context.Background(), http.MethodHead, "/test-head", nil)
	headResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(headResponseRecorder, headRequest)
	if headResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-head, got %d", headResponseRecorder.Code)
	}

	postRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test-post", strings.NewReader(`{"key":"value"}`))
	postRequest.Header.Set("Content-Type", "application/json")
	postResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(postResponseRecorder, postRequest)
	if postResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 from test-post, got %d", postResponseRecorder.Code)
	}

	putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/test-put", strings.NewReader(`{"key":"value"}`))
	putRequest.Header.Set("Content-Type", "application/json")
	putResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(putResponseRecorder, putRequest)
	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-put, got %d", putResponseRecorder.Code)
	}

	deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/test-delete", nil)
	deleteResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from test-delete, got %d", deleteResponseRecorder.Code)
	}

	patchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/test-patch", strings.NewReader(`{"key":"value"}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchResponseRecorder := httptest.NewRecorder()
	router.Mux().ServeHTTP(patchResponseRecorder, patchRequest)
	if patchResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-patch, got %d", patchResponseRecorder.Code)
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
		RouteSDKMethodName("test_helper"),
	)

	// Test calling on raw BaseRoute with nil Responses and Extensions to cover initialization
	emptyBaseRoute := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteBinaryResponse(http.StatusOK, "raw binary")(emptyBaseRoute)
	secondEmptyBaseRoute := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteNoContentResponse("raw no content")(secondEmptyBaseRoute)
	thirdEmptyBaseRoute := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteSDKGroupName("data", "cache")(thirdEmptyBaseRoute)
	RouteSDKMethodName("custom_method")(thirdEmptyBaseRoute)
	// Call again with non-nil Extensions
	RouteSDKGroupName("data", "store")(thirdEmptyBaseRoute)
	RouteSDKMethodName("another_method")(thirdEmptyBaseRoute)
	fourthEmptyBaseRoute := &fuego.BaseRoute{Operation: &openapi3.Operation{}}
	RouteSDKMethodName("standalone_method")(fourthEmptyBaseRoute)
	RouteNoRequestBody()(fourthEmptyBaseRoute)

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

	dummyHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"message":"ok"}`))
	}

	GetRoute[DummyResponse](router, "/test/typed/get", dummyHandler, RouteTag("Test"), RouteSummary("Get Typed"))
	HeadRoute[DummyResponse](router, "/test/typed/head", dummyHandler, RouteTag("Test"), RouteSummary("Head Typed"))
	PostRoute[DummyResponse, DummyRequest](router, "/test/typed/post", dummyHandler, RouteTag("Test"), RouteSummary("Post Typed"))
	PutRoute[DummyResponse, DummyRequest](router, "/test/typed/put", dummyHandler, RouteTag("Test"), RouteSummary("Put Typed"))
	DeleteRoute[DummyResponse](router, "/test/typed/delete", dummyHandler, RouteTag("Test"), RouteSummary("Delete Typed"))
	PatchRoute[DummyResponse, DummyRequest](router, "/test/typed/patch", dummyHandler, RouteTag("Test"), RouteSummary("Patch Typed"))

	// Test HTTP dispatching
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), method, "/test/typed/"+strings.ToLower(method), strings.NewReader(`{"name":"layr"}`))
			request.Header.Set("Content-Type", "application/json")
			responseResponseRecorder := httptest.NewRecorder()
			engine.Mux.ServeHTTP(responseResponseRecorder, request)
			if responseResponseRecorder.Code != http.StatusOK {
				t.Fatalf("expected status 200 for %s, got %d", method, responseResponseRecorder.Code)
			}
		})
	}
}

func TestCoreValidateRouterRoutesComplianceUnit(t *testing.T) {
	dummyHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}

	// 1. Success case
	validRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](validRouter, "/valid", dummyHandler,
		RouteOperationID("service__sub_module__action_name"),
		RouteSDKGroupName("service", "sub_module"),
		RouteSDKMethodName("action_name"),
	)
	AssertRouterRoutesCompliance(t, validRouter)

	// 1b. Success case with []any group extensions
	anyGroupRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](anyGroupRouter, "/any-group", dummyHandler,
		RouteOperationID("service__sub_module__action_name"),
		func(baseRoute *fuego.BaseRoute) {
			if baseRoute.Operation.Extensions == nil {
				baseRoute.Operation.Extensions = make(map[string]any)
			}
			baseRoute.Operation.Extensions["x-sdk-group-name"] = []any{"service", "sub_module"}
		},
		RouteSDKMethodName("action_name"),
	)
	AssertRouterRoutesCompliance(t, anyGroupRouter)

	// 2. Missing OperationID
	noOpRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](noOpRouter, "/no-op", dummyHandler)
	for _, pathItem := range noOpRouter.OutputOpenAPISpec().Paths.Map() {
		for _, op := range pathItem.Operations() {
			op.OperationID = ""
		}
	}
	if err := ValidateRouterRoutesCompliance(noOpRouter); err == nil || !strings.Contains(err.Error(), "missing RouteOperationID") {
		t.Fatalf("expected missing RouteOperationID error, got: %v", err)
	}

	// 3. Non-snake_case OperationID segment
	invalidOpRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](invalidOpRouter, "/invalid-op", dummyHandler,
		RouteOperationID("service__invalidAction"),
	)
	if err := ValidateRouterRoutesCompliance(invalidOpRouter); err == nil || !strings.Contains(err.Error(), "must be snake_case") {
		t.Fatalf("expected non-snake_case error, got: %v", err)
	}

	// 4. Missing SDK Group Name
	noGroupRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](noGroupRouter, "/no-group", dummyHandler,
		RouteOperationID("service__group__action"),
	)
	if err := ValidateRouterRoutesCompliance(noGroupRouter); err == nil || !strings.Contains(err.Error(), "missing RouteSDKGroupName") {
		t.Fatalf("expected missing RouteSDKGroupName error, got: %v", err)
	}

	// 5. SDK Group Name length mismatch
	lenMismatchRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](lenMismatchRouter, "/len-mismatch", dummyHandler,
		RouteOperationID("service__group__action"),
		RouteSDKGroupName("service"),
		RouteSDKMethodName("action"),
	)
	if err := ValidateRouterRoutesCompliance(lenMismatchRouter); err == nil || !strings.Contains(err.Error(), "length mismatch") {
		t.Fatalf("expected length mismatch error, got: %v", err)
	}

	// 6. SDK Group Name content mismatch
	contentMismatchRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](contentMismatchRouter, "/content-mismatch", dummyHandler,
		RouteOperationID("service__group__action"),
		RouteSDKGroupName("service", "other"),
		RouteSDKMethodName("action"),
	)
	if err := ValidateRouterRoutesCompliance(contentMismatchRouter); err == nil || !strings.Contains(err.Error(), "group name mismatch") {
		t.Fatalf("expected group name mismatch error, got: %v", err)
	}

	// 7. Missing SDK Method Name
	noMethodRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](noMethodRouter, "/no-method", dummyHandler,
		RouteOperationID("service__action"),
		RouteSDKGroupName("service"),
	)
	if err := ValidateRouterRoutesCompliance(noMethodRouter); err == nil || !strings.Contains(err.Error(), "missing RouteSDKMethodName") {
		t.Fatalf("expected missing RouteSDKMethodName error, got: %v", err)
	}

	// 8. Non-string SDK Method Name
	nonStringMethodRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](nonStringMethodRouter, "/non-string-method", dummyHandler,
		RouteOperationID("service__action"),
		RouteSDKGroupName("service"),
		func(baseRoute *fuego.BaseRoute) {
			baseRoute.Operation.Extensions["x-sdk-method-name"] = 123
		},
	)
	if err := ValidateRouterRoutesCompliance(nonStringMethodRouter); err == nil || !strings.Contains(err.Error(), "must be string") {
		t.Fatalf("expected non-string method error, got: %v", err)
	}

	// 9. SDK Method Name value mismatch
	methodMismatchRouter := NewRouter(fuego.NewServer(fuego.WithAddr("localhost:0")))
	GetRoute[string](methodMismatchRouter, "/method-mismatch", dummyHandler,
		RouteOperationID("service__action"),
		RouteSDKGroupName("service"),
		RouteSDKMethodName("wrong_action"),
	)
	if err := ValidateRouterRoutesCompliance(methodMismatchRouter); err == nil || !strings.Contains(err.Error(), "method name mismatch") {
		t.Fatalf("expected method name mismatch error, got: %v", err)
	}

	mockTestReporter := &mockTestReporter{}
	AssertRouterRoutesCompliance(mockTestReporter, methodMismatchRouter)
	if !mockTestReporter.failed {
		t.Fatal("expected mockTestReporter.Fatal to be called")
	}
}

type mockTestReporter struct {
	testing.TB
	failed bool
}

func (mock *mockTestReporter) Helper() {}

func (mock *mockTestReporter) Fatal(args ...any) {
	mock.failed = true
}
