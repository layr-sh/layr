package core

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestCoreServerAllEndpointsUnit(t *testing.T) {
	config := DefaultConfig()
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, err := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})
	if server.Kernel() == nil {
		t.Fatal("expected non-nil Kernel from server.Kernel()")
	}
	AssertRouterRoutesCompliance(t, server.BaseRouter())
	AssertRouterRoutesCompliance(t, server.ControlPlaneRouter())

	// Test /healthz
	healthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	healthResponseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(healthResponseRecorder, healthRequest)

	if healthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", healthResponseRecorder.Code)
	}

	var healthResponse map[string]any
	if unmarshalErr := json.Unmarshal(healthResponseRecorder.Body.Bytes(), &healthResponse); unmarshalErr != nil {
		t.Fatalf("failed to decode health response: %v", unmarshalErr)
	}
	if healthResponse["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", healthResponse["status"])
	}

	// Test /metrics
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsResponseRecorder, metricsRequest)

	if metricsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /metrics 200, got %d", metricsResponseRecorder.Code)
	}
	metricsBody := metricsResponseRecorder.Body.String()
	if !strings.Contains(metricsBody, "uptime_seconds") || !strings.Contains(metricsBody, "http_requests_total") {
		t.Fatalf("metrics body missing expected prometheus metrics: %s", metricsBody)
	}

	// Test /v1/manifest without publishable key -> 200 (System Discovery is public & unauthenticated)
	noKeyManifestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/manifest", nil)
	noKeyManifestResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(noKeyManifestResponseRecorder, noKeyManifestRequest)
	if noKeyManifestResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected manifest 200 without key, got %d", noKeyManifestResponseRecorder.Code)
	}

	// Register a base client test endpoint on router
	GetRoute[string](server.BaseRouter(), "/v1/client-test", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-client"))
	})

	// Test /v1/function route without publishable key -> 200 (not required for functions)
	GetRoute[string](server.BaseRouter(), "/v1/function/{endpoint_name}", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-function"))
	})
	noKeyFunctionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/function/public-fn", nil)
	noKeyFunctionResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(noKeyFunctionResponseRecorder, noKeyFunctionRequest)
	if noKeyFunctionResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected function endpoint 200 without key, got %d", noKeyFunctionResponseRecorder.Code)
	}

	// Test base client endpoint without publishable key -> 401
	noKeyClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/client-test", nil)
	noKeyClientResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(noKeyClientResponseRecorder, noKeyClientRequest)
	if noKeyClientResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected client endpoint 401 without key, got %d", noKeyClientResponseRecorder.Code)
	}

	// Test base client endpoint with valid publishable key -> 200
	publishableKey := cryptoKeyManager.DerivePublishableKey()
	clientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/client-test", nil)
	clientRequest.Header.Set("Publishable-Key", publishableKey)
	clientResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientResponseRecorder, clientRequest)
	if clientResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with valid key, got %d", clientResponseRecorder.Code)
	}

	// Test fallback X-Publishable-Key header -> 200
	fallbackClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/client-test", nil)
	fallbackClientRequest.Header.Set("X-Publishable-Key", publishableKey)
	fallbackClientResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(fallbackClientResponseRecorder, fallbackClientRequest)
	if fallbackClientResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with fallback key, got %d", fallbackClientResponseRecorder.Code)
	}

	// Test base client endpoint with valid authenticated JWT in Authorization header -> 200
	validJWTToken := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: "01918a24-7777-7000-8000-000000000001",
		Role:    "authenticated",
	})
	clientAuthHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/client-test", nil)
	clientAuthHeaderRequest.Header.Set("Authorization", "Bearer "+validJWTToken)
	clientAuthHeaderResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientAuthHeaderResponseRecorder, clientAuthHeaderRequest)
	if clientAuthHeaderResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with valid JWT authorization, got %d", clientAuthHeaderResponseRecorder.Code)
	}

	// Test base client endpoint with authenticated context (e.g. verified session or service account) -> 200
	authenticatedCtx := WithAuthContext(context.Background(), AuthContext{
		ServiceAccountID: "01918a24-7777-7000-8000-000000000002",
	})
	clientAuthContextRequest := httptest.NewRequestWithContext(authenticatedCtx, http.MethodGet, "/v1/client-test", nil)
	clientAuthContextResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientAuthContextResponseRecorder, clientAuthContextRequest)
	if clientAuthContextResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with authenticated context, got %d", clientAuthContextResponseRecorder.Code)
	}

	// Test base client endpoint with invalid Authorization header -> 401 Unauthorized
	clientInvalidAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/client-test", nil)
	clientInvalidAuthRequest.Header.Set("Authorization", "Bearer invalid.signature.token")
	clientInvalidAuthResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientInvalidAuthResponseRecorder, clientInvalidAuthRequest)
	if clientInvalidAuthResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected client endpoint 401 with invalid auth header, got %d", clientInvalidAuthResponseRecorder.Code)
	}

	// Test Public OpenAPI /v1/spec.json
	jsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/spec.json", nil)
	jsonSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(jsonSpecResponseRecorder, jsonSpecRequest)
	if jsonSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/spec.json 200, got %d", jsonSpecResponseRecorder.Code)
	}
	if !strings.Contains(jsonSpecResponseRecorder.Body.String(), "Layr Client API Engine") {
		t.Fatalf("expected public openapi spec in response: %s", jsonSpecResponseRecorder.Body.String())
	}

	// Test Public OpenAPI /v1/spec.yaml
	yamlSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/spec.yaml", nil)
	yamlSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(yamlSpecResponseRecorder, yamlSpecRequest)
	if yamlSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/spec.yaml 200, got %d", yamlSpecResponseRecorder.Code)
	}
	if !strings.Contains(yamlSpecResponseRecorder.Body.String(), "Layr Client API Engine") {
		t.Fatalf("expected public openapi 3.1.0 in yaml response: %s", yamlSpecResponseRecorder.Body.String())
	}

	// Test Control Plane OpenAPI /v1/_/spec.json
	controlPlaneJsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/spec.json", nil)
	controlPlaneJsonSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaneJsonSpecResponseRecorder, controlPlaneJsonSpecRequest)
	if controlPlaneJsonSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/_/spec.json 200, got %d", controlPlaneJsonSpecResponseRecorder.Code)
	}
	if !strings.Contains(controlPlaneJsonSpecResponseRecorder.Body.String(), "Layr Control Plane API Engine") {
		t.Fatalf("expected control plane openapi spec in response: %s", controlPlaneJsonSpecResponseRecorder.Body.String())
	}

	// Test Control Plane OpenAPI /v1/_/spec.yaml
	controlPlaneYamlRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/spec.yaml", nil)
	controlPlaneYamlResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaneYamlResponseRecorder, controlPlaneYamlRequest)
	if controlPlaneYamlResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/_/spec.yaml 200, got %d", controlPlaneYamlResponseRecorder.Code)
	}
	if !strings.Contains(controlPlaneYamlResponseRecorder.Body.String(), "Layr Control Plane API Engine") {
		t.Fatalf("expected control plane openapi 3.1.0 in yaml response: %s", controlPlaneYamlResponseRecorder.Body.String())
	}
}

func TestCoreServerStartAndShutdownErrorsUnit(t *testing.T) {
	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSigner := NewJWTSigner(cryptoKeyManager)

	// 1. Test Start() error with invalid listen address
	invalidAddressServer := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})
	invalidAddressServer.server.Addr = "invalid:address:too:many:colons"
	if err := invalidAddressServer.Start(); err == nil {
		t.Fatal("expected error from Start with invalid listen address, got nil")
	}

	// 2. Test Shutdown() error when context expires with in-flight connection
	drainServer := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})
	handlerStartedChannel := make(chan struct{})
	unblockHandlerChannel := make(chan struct{})

	GetRoute[string](drainServer.BaseRouter(), "/v1/blocking-test", func(responseWriter http.ResponseWriter, request *http.Request) {
		close(handlerStartedChannel)
		<-unblockHandlerChannel
		responseWriter.WriteHeader(http.StatusOK)
	})

	var listenConfig net.ListenConfig
	tcpListener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open tcp listener: %v", err)
	}

	go func() {
		_ = drainServer.server.Serve(tcpListener)
	}()

	go func() {
		inFlightRequest, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+tcpListener.Addr().String()+"/v1/blocking-test", nil)
		if requestErr == nil {
			inFlightRequest.Header.Set("Publishable-Key", cryptoKeyManager.DerivePublishableKey())
			response, clientErr := http.DefaultClient.Do(inFlightRequest)
			if clientErr == nil {
				_ = response.Body.Close()
			}
		}
	}()

	<-handlerStartedChannel

	// In-flight connection is active, shutdown with an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	shutdownErr := drainServer.Shutdown(ctx)
	close(unblockHandlerChannel)

	if shutdownErr == nil {
		t.Fatal("expected error from Shutdown with canceled context on active connection, got nil")
	}
}

func TestCoreServerPanicRecoveryUnit(t *testing.T) {
	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})
	GetRoute[string](server.BaseRouter(), "/v1/panic-endpoint", func(responseWriter http.ResponseWriter, request *http.Request) {
		panic("simulated unhandled panic in handler")
	})

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/panic-endpoint", nil)
	request.Header.Set("Publishable-Key", cryptoKeyManager.DerivePublishableKey())
	responseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 from panic recovery, got %d", responseRecorder.Code)
	}
	if !strings.Contains(responseRecorder.Body.String(), "Service temporarily unavailable") {
		t.Fatalf("expected body to contain Service temporarily unavailable detail, got: %s", responseRecorder.Body.String())
	}
}

func TestCoreServerMiddlewareAuthContextPopulationUnit(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})

	var capturedAuthContext AuthContext
	var capturedEventActor EventActor

	testHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		capturedAuthContext = GetAuthContext(request.Context())
		capturedEventActor, _ = GetEventActor(request.Context())
		responseWriter.WriteHeader(http.StatusOK)
	})

	middlewareHandler := server.middleware(testHandler)

	// 1. Unauthenticated request (no headers, no cookies)
	unauthenticatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	unauthenticatedResponseRecorder := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(unauthenticatedResponseRecorder, unauthenticatedRequest)

	if capturedAuthContext.UserID != "" || capturedAuthContext.JWT.Subject != "" || capturedAuthContext.JWT.Role != "" {
		t.Fatalf("expected empty auth context for unauthenticated request, got: %+v", capturedAuthContext)
	}
	if capturedEventActor.Type != "" {
		t.Fatalf("expected empty event actor for unauthenticated request, got: %+v", capturedEventActor)
	}

	// 2. User Access Token via Bearer
	userID := uuid.NewV7().String()
	userToken := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: userID,
		Role:    "authenticated",
	}, 600)

	userRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	userRequest.Header.Set("Authorization", "Bearer "+userToken)
	userRequest.Header.Set("Refresh-Token", "test-refresh-token")
	userResponseRecorder := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(userResponseRecorder, userRequest)

	if capturedAuthContext.JWT.Subject != userID || capturedAuthContext.UserID != userID || capturedAuthContext.ServiceAccountID != "" {
		t.Fatalf("expected subject and UserID %s with empty ServiceAccountID, got %+v", userID, capturedAuthContext)
	}
	if !capturedAuthContext.IsUser() || capturedAuthContext.IsServiceAccount() {
		t.Fatalf("expected user flags for user token, got: %+v", capturedAuthContext)
	}
	expectedRefreshHash := jwtSigner.HashRefreshToken("test-refresh-token")
	if capturedAuthContext.RefreshTokenHash != expectedRefreshHash {
		t.Fatalf("expected refresh token hash %s, got %s", expectedRefreshHash, capturedAuthContext.RefreshTokenHash)
	}
	if capturedEventActor.Type != "user" || capturedEventActor.ID == nil || capturedEventActor.ID.String() != userID {
		t.Fatalf("expected user actor for %s, got: %+v", userID, capturedEventActor)
	}

	// 2b. User Access Token with Cookie session tokens
	userCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	userCookieRequest.Header.Set("Authorization", "Bearer "+userToken)
	userCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameSecure, Value: "secure-refresh-cookie"})
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), userCookieRequest)
	if capturedAuthContext.RefreshTokenHash != jwtSigner.HashRefreshToken("secure-refresh-cookie") {
		t.Fatalf("expected secure cookie refresh hash, got: %s", capturedAuthContext.RefreshTokenHash)
	}

	userInsecureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	userInsecureCookieRequest.Header.Set("Authorization", "Bearer "+userToken)
	userInsecureCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameInsecure, Value: "insec-refresh-cookie"})
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), userInsecureCookieRequest)
	if capturedAuthContext.RefreshTokenHash != jwtSigner.HashRefreshToken("insec-refresh-cookie") {
		t.Fatalf("expected insecure cookie refresh hash, got: %s", capturedAuthContext.RefreshTokenHash)
	}

	userFallbackRefreshRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	userFallbackRefreshRequest.Header.Set("Authorization", "Bearer "+userToken)
	userFallbackRefreshRequest.Header.Set("X-Refresh-Token", "fallback-refresh-token")
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), userFallbackRefreshRequest)
	if capturedAuthContext.RefreshTokenHash != jwtSigner.HashRefreshToken("fallback-refresh-token") {
		t.Fatalf("expected fallback refresh token hash, got: %s", capturedAuthContext.RefreshTokenHash)
	}

	// 3. M2M JWT Token via Bearer
	serviceAccountID := uuid.NewV7().String()
	slugifier := NewSlugifier()
	handle := slugifier.Slugify(GetConfig().Project.Name)
	if handle == "" {
		handle = "layr"
	}
	layrAudience := handle + ":service_account"
	m2mToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, []string{"data:read"}, 600, layrAudience)
	if err != nil {
		t.Fatalf("failed to generate m2m token: %v", err)
	}

	m2mRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	m2mRequest.Header.Set("Authorization", "Bearer "+m2mToken)
	m2mResponseRecorder := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(m2mResponseRecorder, m2mRequest)

	if capturedAuthContext.JWT.Subject != serviceAccountID || capturedAuthContext.JWT.Role != "service_role" {
		t.Fatalf("expected m2m auth context, got: %+v", capturedAuthContext)
	}
	if capturedAuthContext.UserID != "" {
		t.Fatalf("service account must never populate UserID: got %q", capturedAuthContext.UserID)
	}
	if capturedAuthContext.ServiceAccountID != serviceAccountID {
		t.Fatalf("expected ServiceAccountID %s, got: %+v", serviceAccountID, capturedAuthContext)
	}
	if capturedAuthContext.IsUser() || !capturedAuthContext.IsServiceAccount() {
		t.Fatalf("expected service account flags for m2m token, got: %+v", capturedAuthContext)
	}
	if capturedEventActor.Type != "service_account" || capturedEventActor.ID == nil || capturedEventActor.ID.String() != serviceAccountID {
		t.Fatalf("expected service_account actor, got: %+v", capturedEventActor)
	}

	// 3b. Foreign audience M2M token rejected
	foreignToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, []string{"invoices:read"}, 600, "https://billing.example.com")
	if err != nil {
		t.Fatalf("failed to generate foreign m2m token: %v", err)
	}
	foreignRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	foreignRequest.Header.Set("Authorization", "Bearer "+foreignToken)
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), foreignRequest)
	if capturedAuthContext.ServiceAccountID != "" || capturedAuthContext.UserID != "" {
		t.Fatalf("foreign audience token must be rejected: got %+v", capturedAuthContext)
	}

	// 3c. Empty project name fallback audience "layr:service_account"
	serverConfig := DefaultConfig()
	serverConfig.Project.Name = ""
	SetLoadedConfig(serverConfig)
	fallbackM2MToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, []string{"data:read"}, 600, "layr:service_account")
	if err != nil {
		UnloadConfig()
		t.Fatalf("failed to generate fallback m2m token: %v", err)
	}
	fallbackM2MRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	fallbackM2MRequest.Header.Set("Authorization", "Bearer "+fallbackM2MToken)
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), fallbackM2MRequest)
	UnloadConfig()
	if capturedAuthContext.ServiceAccountID != serviceAccountID {
		t.Fatalf("expected fallback audience token accepted, got: %+v", capturedAuthContext)
	}

	// 4. Invalid JWT fallback to unauthenticated
	badRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	badRequest.Header.Set("Authorization", "Bearer invalid.jwt.token")
	badResponseRecorder := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(badResponseRecorder, badRequest)

	if capturedAuthContext.UserID != "" || capturedAuthContext.JWT.Subject != "" || capturedAuthContext.JWT.Role != "" {
		t.Fatalf("expected empty auth context on invalid token, got: %+v", capturedAuthContext)
	}
	if capturedEventActor.Type != "" {
		t.Fatalf("expected empty event actor on invalid token, got: %+v", capturedEventActor)
	}

	// 5. Authenticated Anonymous User Account (user with is_anonymous = true)
	anonUserID := uuid.NewV7().String()
	anonToken := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 600)

	anonUserRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	anonUserRequest.Header.Set("Authorization", "Bearer "+anonToken)
	middlewareHandler.ServeHTTP(httptest.NewRecorder(), anonUserRequest)

	if capturedAuthContext.JWT.Subject != anonUserID || !capturedAuthContext.JWT.IsAnonymous || capturedAuthContext.JWT.Role != "authenticated" {
		t.Fatalf("expected authenticated anonymous user context, got: %+v", capturedAuthContext)
	}
	if capturedEventActor.Type != "user" || capturedEventActor.ID == nil || capturedEventActor.ID.String() != anonUserID {
		t.Fatalf("expected user event actor for anonymous user, got: %+v", capturedEventActor)
	}
}

func TestCoreWriteJSONResponseUnit(t *testing.T) {
	// Test WriteJSONResponse
	jsonResponseRecorder := httptest.NewRecorder()
	payload := map[string]string{"message": "hello world"}
	WriteJSONResponse(jsonResponseRecorder, http.StatusCreated, payload)

	if jsonResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", jsonResponseRecorder.Code)
	}
	if contentType := jsonResponseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", contentType)
	}

	var decoded map[string]string
	if err := json.Unmarshal(jsonResponseRecorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if decoded["message"] != "hello world" {
		t.Fatalf("expected hello world, got %s", decoded["message"])
	}
}

func TestCoreServerHostHandlerUnit(t *testing.T) {
	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})

	if len(server.HostHandlers()) != 0 {
		t.Fatal("expected 0 host handlers initially")
	}
	server.RegisterHostHandler(nil)
	if len(server.HostHandlers()) != 0 {
		t.Fatal("expected nil host handler ignored")
	}

	testHostHandler := func(responseWriter http.ResponseWriter, request *http.Request) bool {
		if request.Host == "subdomain.example.com" {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("handled-subdomain"))
			return true
		}
		return false
	}
	server.RegisterHostHandler(testHostHandler)
	if len(server.HostHandlers()) != 1 {
		t.Fatalf("expected 1 host handler, got %d", len(server.HostHandlers()))
	}

	// Test request matching host handler
	matchedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/any/subpath", nil)
	matchedRequest.Host = "subdomain.example.com"
	matchedResponseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(matchedResponseRecorder, matchedRequest)
	if matchedResponseRecorder.Code != http.StatusOK || matchedResponseRecorder.Body.String() != "handled-subdomain" {
		t.Fatalf("expected host handler to intercept, got code=%d body=%s", matchedResponseRecorder.Code, matchedResponseRecorder.Body.String())
	}

	// Test request with unhandled host falls through to standard 404
	unmatchedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/unhandled", nil)
	unmatchedRequest.Host = "other.example.com"
	unmatchedResponseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(unmatchedResponseRecorder, unmatchedRequest)
	if unmatchedResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from unhandled route, got %d", unmatchedResponseRecorder.Code)
	}
}

func generateTestCertAndKeyPEM(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Layr Test"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost", "layr.test"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certBuf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBuf := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return string(certBuf), string(keyBuf)
}

func TestCoreServerValidateHostUnit(t *testing.T) {
	ctx := context.Background()

	testConfig := DefaultConfig()
	testConfig.Server.BaseURL = "https://app.layr.sh:5297"
	testConfig.Server.TLS.Domains = []string{"static.layr.sh", "api.layr.sh", "custom-port.layr.sh:443"}
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)

	// Register dynamic host validator for custom domains
	var customValidatorInvoked bool
	server.RegisterHostValidator(func(validatorCtx context.Context, host string) (bool, error) {
		customValidatorInvoked = true
		if host == "custom.myworker.com" {
			return true, nil
		}
		if host == "error.domain.com" {
			return false, errors.New("validator database failure")
		}
		return false, nil
	})

	// 1. Empty host returns false
	valid, err := server.ValidateHost(ctx, "")
	require.NoError(t, err)
	require.False(t, valid)

	// 2. BaseURL hostname matches
	valid, err = server.ValidateHost(ctx, "app.layr.sh")
	require.NoError(t, err)
	require.True(t, valid)

	// 3. BaseURL with port in Host header matches
	valid, err = server.ValidateHost(ctx, "APP.LAYR.SH:443")
	require.NoError(t, err)
	require.True(t, valid)

	// 4. Static domains in Server.TLS.Domains match
	valid, err = server.ValidateHost(ctx, "static.layr.sh")
	require.NoError(t, err)
	require.True(t, valid)

	valid, err = server.ValidateHost(ctx, "API.LAYR.SH:5297")
	require.NoError(t, err)
	require.True(t, valid)

	valid, err = server.ValidateHost(ctx, "custom-port.layr.sh")
	require.NoError(t, err)
	require.True(t, valid)

	// 5. Dynamic host validator matches
	customValidatorInvoked = false
	valid, err = server.ValidateHost(ctx, "custom.myworker.com")
	require.NoError(t, err)
	require.True(t, valid)
	require.True(t, customValidatorInvoked)

	// 6. Unknown domain is rejected
	valid, err = server.ValidateHost(ctx, "unknown-attacker.evil.com")
	require.NoError(t, err)
	require.False(t, valid)

	// 7. Error from validator propagates
	valid, err = server.ValidateHost(ctx, "error.domain.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "validator database failure")
	require.False(t, valid)

	// 8. RegisterHostValidator with nil is a no-op
	server.RegisterHostValidator(nil)
	require.Len(t, server.HostValidators(), 1)

	// 9. HostPolicy returns nil for permitted host, ErrHostNotPermitted for unpermitted, and error on failure
	require.NoError(t, server.HostPolicy(ctx, "app.layr.sh"))
	unpermittedErr := server.HostPolicy(ctx, "unknown-attacker.evil.com")
	require.Error(t, unpermittedErr)
	require.ErrorIs(t, unpermittedErr, ErrHostNotPermitted)
	validatorErr := server.HostPolicy(ctx, "error.domain.com")
	require.Error(t, validatorErr)
	require.Contains(t, validatorErr.Error(), "validator database failure")
}

func TestCoreServerTLSConfigManualPEMUnit(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertPEM = certPEM
	testConfig.Server.TLS.KeyPEM = keyPEM
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	server := NewServer(kernel)
	require.NotNil(t, server)
	require.Nil(t, server.AutocertManager())
	require.NotNil(t, server.server.TLSConfig)
	require.Len(t, server.server.TLSConfig.Certificates, 1)

	// Verify manual TLS GetCertificate resolves cert from DBCertCache
	getCert := server.server.TLSConfig.GetCertificate
	require.NotNil(t, getCert)

	// nil client hello returns nil, nil
	certificate, err := getCert(nil)
	require.NoError(t, err)
	require.Nil(t, certificate)

	// empty ServerName returns nil, nil
	certificate, err = getCert(&tls.ClientHelloInfo{ServerName: ""})
	require.NoError(t, err)
	require.Nil(t, certificate)

	// Unknown ServerName (not in cache) returns nil, nil (falls back to default cert)
	certificate, err = getCert(&tls.ClientHelloInfo{ServerName: "unknown.domain.com"})
	require.NoError(t, err)
	require.Nil(t, certificate)

	// ServerName found in DBCertCache returns the custom cert
	customCertPEM, customKeyPEM := generateTestCertAndKeyPEM(t)
	combinedPEM := customCertPEM + "\n" + customKeyPEM
	require.NoError(t, server.DBCertCache().Put(context.Background(), "custom.domain.com", []byte(combinedPEM)))
	certificate, err = getCert(&tls.ClientHelloInfo{ServerName: "CUSTOM.DOMAIN.COM"})
	require.NoError(t, err)
	require.NotNil(t, certificate)

	// ServerName with trailing dot and port resolves correctly
	portCertificate, portErr := getCert(&tls.ClientHelloInfo{ServerName: "CUSTOM.DOMAIN.COM.:443"})
	require.NoError(t, portErr)
	require.NotNil(t, portCertificate)

	// Corrupted cert bytes in cache falls back to nil, nil
	require.NoError(t, server.DBCertCache().Put(context.Background(), "corrupt.domain.com", []byte("invalid-cert-bytes")))
	certificate, err = getCert(&tls.ClientHelloInfo{ServerName: "corrupt.domain.com"})
	require.NoError(t, err)
	require.Nil(t, certificate)
}

func TestCoreServerTLSConfigManualFilesUnit(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	require.NoError(t, os.WriteFile(certPath, []byte(certPEM), 0o600))
	require.NoError(t, os.WriteFile(keyPath, []byte(keyPEM), 0o600))

	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertFile = certPath
	testConfig.Server.TLS.KeyFile = keyPath
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	require.NotNil(t, server)
	require.NotNil(t, server.server.TLSConfig)
	require.Len(t, server.server.TLSConfig.Certificates, 1)
}

func TestCoreServerTLSConfigAutoTLSUnit(t *testing.T) {
	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = true
	testConfig.Server.TLS.AcmeEmail = "admin@layr.sh"
	testConfig.Server.TLS.AcmeStaging = true
	testConfig.Server.TLS.Domains = []string{"layr.test"}
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	require.NotNil(t, server)
	require.NotNil(t, server.server.TLSConfig)
	require.NotNil(t, server.AutocertManager())
	require.NotNil(t, server.server.TLSConfig.GetCertificate)

	// Test autocert manager HostPolicy
	hostPolicy := server.AutocertManager().HostPolicy
	require.NotNil(t, hostPolicy)

	// Allowed domain
	require.NoError(t, hostPolicy(context.Background(), "layr.test"))

	// Rejected domain
	rejectErr := hostPolicy(context.Background(), "unknown.evil.com")
	require.Error(t, rejectErr)
	require.True(t, errors.Is(rejectErr, ErrHostNotPermitted))

	// Error from dynamic host validator in HostPolicy
	server.RegisterHostValidator(func(ctx context.Context, host string) (bool, error) {
		return false, errors.New("validator database failure")
	})
	validatorErr := hostPolicy(context.Background(), "validator.fail.com")
	require.Error(t, validatorErr)
	require.Contains(t, validatorErr.Error(), "validator database failure")
}

func TestCoreServerTLSDisabledUnit(t *testing.T) {
	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = false
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	require.NotNil(t, server)
	require.Nil(t, server.server.TLSConfig)
	require.Nil(t, server.TLSConfig())
	require.NotNil(t, server.DBCertCache())
	require.False(t, server.TLSEnabled())
	require.Nil(t, server.AutocertManager())
	require.Empty(t, server.HostValidators())
}

func TestCoreServerTLSConfigAutoTLSWithKernelDBUnit(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = true
	testConfig.Server.TLS.AcmeEmail = "admin@layr.sh"
	testConfig.Server.TLS.Domains = []string{"layr.test"}
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	server := NewServer(kernel)
	require.NotNil(t, server)
	require.NotNil(t, server.TLSConfig())
	require.NotNil(t, server.DBCertCache())
}

func TestCoreServerTLSInvalidManualCertUnit(t *testing.T) {
	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertPEM = "invalid-cert-pem"
	testConfig.Server.TLS.KeyPEM = "invalid-key-pem"
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	require.NotNil(t, server)
	require.Nil(t, server.TLSConfig())

	startErr := server.Start()
	require.Error(t, startErr)
	require.Contains(t, startErr.Error(), "TLS is enabled but TLS configuration failed to initialize")
}

func TestCoreServerHTTPToHTTPSRedirectUnit(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	testConfig := DefaultConfig()
	testConfig.Server.BaseURL = "https://app.layr.sh:8443"
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertPEM = certPEM
	testConfig.Server.TLS.KeyPEM = keyPEM
	testConfig.Server.TLS.HTTPRedirect = true
	testConfig.Server.TLS.HTTPListenAddr = ":8080"
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	require.True(t, server.TLSEnabled())
	require.NotNil(t, server.HTTPServer())
	require.Equal(t, ":8080", server.HTTPServer().Addr)

	httpHandler := server.HTTPServer().Handler
	require.NotNil(t, httpHandler)

	// 1. Valid host redirect with port stripping
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://app.layr.sh:8080/v1/users?page=1", nil)
	request.Host = "app.layr.sh:8080"
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusMovedPermanently, responseRecorder.Code)
	require.Equal(t, "https://app.layr.sh/v1/users?page=1", responseRecorder.Header().Get("Location"))

	// 2. Unknown host falls back to BaseURL with custom port preserved
	unknownRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://evil-attacker.com/v1/data", nil)
	unknownRequest.Host = "evil-attacker.com"
	unknownResponseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(unknownResponseRecorder, unknownRequest)

	require.Equal(t, http.StatusMovedPermanently, unknownResponseRecorder.Code)
	require.Equal(t, "https://app.layr.sh:8443/v1/data", unknownResponseRecorder.Header().Get("Location"))

	// 3. AutoTLS enabled wraps with autocert HTTPHandler
	testConfig.Server.TLS.AutoTLS = true
	testConfig.Server.TLS.Domains = []string{"app.layr.sh"}
	autoServer := NewServer(kernel)
	require.NotNil(t, autoServer.HTTPServer())
	require.NotNil(t, autoServer.HTTPServer().Handler)

	autoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://app.layr.sh/v1/ping", nil)
	autoRequest.Host = "app.layr.sh"
	autoResponseRecorder := httptest.NewRecorder()
	autoServer.HTTPServer().Handler.ServeHTTP(autoResponseRecorder, autoRequest)
	require.Equal(t, http.StatusMovedPermanently, autoResponseRecorder.Code)

	// 4. Default HTTPListenAddr when empty
	testConfig.Server.TLS.HTTPListenAddr = ""
	defaultHTTPServer := NewServer(kernel)
	require.NotNil(t, defaultHTTPServer.HTTPServer())
	require.Equal(t, ":80", defaultHTTPServer.HTTPServer().Addr)

	// 5. Host handler intercepted on HTTP listener (e.g. plain HTTP custom domain without TLS)
	var hostHandlerCalled bool
	server.RegisterHostHandler(func(responseWriter http.ResponseWriter, hostRequest *http.Request) bool {
		if hostRequest.Host == "plain.domain.com" {
			hostHandlerCalled = true
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("plain-custom-domain"))
			return true
		}
		return false
	})
	plainRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://plain.domain.com/data", nil)
	plainRequest.Host = "plain.domain.com"
	plainResponseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(plainResponseRecorder, plainRequest)
	require.Equal(t, http.StatusOK, plainResponseRecorder.Code)
	require.Equal(t, "plain-custom-domain", plainResponseRecorder.Body.String())
	require.True(t, hostHandlerCalled)

	// 6. BaseURL is plain HTTP (core server has no TLS) -> passes to server.server.Handler
	plainBaseURLConfig := DefaultConfig()
	plainBaseURLConfig.Server.BaseURL = "http://localhost:5297"
	plainBaseURLConfig.Server.TLS.Enabled = true
	plainBaseURLConfig.Server.TLS.AutoTLS = false
	plainBaseURLConfig.Server.TLS.CertPEM = certPEM
	plainBaseURLConfig.Server.TLS.KeyPEM = keyPEM
	plainBaseURLConfig.Server.TLS.HTTPRedirect = true
	plainBaseURLConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(plainBaseURLConfig)

	plainBaseURLServer := NewServer(kernel)
	plainBaseURLHandler := plainBaseURLServer.HTTPServer().Handler
	coreRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/healthz", nil)
	coreRequest.Host = "localhost"
	coreResponseRecorder := httptest.NewRecorder()
	plainBaseURLHandler.ServeHTTP(coreResponseRecorder, coreRequest)
	require.Equal(t, http.StatusOK, coreResponseRecorder.Code)
}

func TestCoreServerHTTPListenerAppliesMiddlewareUnit(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	testConfig := DefaultConfig()
	testConfig.Server.BaseURL = "https://app.layr.sh"
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertPEM = certPEM
	testConfig.Server.TLS.KeyPEM = keyPEM
	testConfig.Server.TLS.HTTPRedirect = true
	testConfig.Server.TLS.HTTPListenAddr = ":8080"
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)

	jwtSigner := kernel.JWTSigner()
	testUserID := uuid.NewV7().String()
	token := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: testUserID,
		Role:    "authenticated",
	})

	var capturedAuthContext AuthContext
	var capturedEventContext EventContext
	var handlerCalled bool

	server.RegisterHostHandler(func(responseWriter http.ResponseWriter, hostRequest *http.Request) bool {
		if hostRequest.Host == "plain.domain.com" {
			handlerCalled = true
			capturedAuthContext = GetAuthContext(hostRequest.Context())
			capturedEventContext, _ = GetEventContext(hostRequest.Context())
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("ok"))
			return true
		}
		return false
	})

	httpHandler := server.HTTPServer().Handler
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://plain.domain.com/data", nil)
	request.Host = "plain.domain.com"
	request.Header.Set("Authorization", "Bearer "+token)
	responseRecorder := httptest.NewRecorder()

	httpHandler.ServeHTTP(responseRecorder, request)
	require.True(t, handlerCalled)
	require.Equal(t, http.StatusOK, responseRecorder.Code)

	// Verify middleware enrichment was executed
	require.NotNil(t, capturedEventContext.RequestID, "middleware should populate EventContext on HTTP listener")
	require.True(t, capturedAuthContext.IsAuthenticated(), "middleware should authenticate JWT on HTTP listener")
	require.Equal(t, testUserID, capturedAuthContext.UserID)
}

func TestCoreServerStartTLSUnit(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKeyPEM(t)

	testConfig := DefaultConfig()
	testConfig.Server.TLS.Enabled = true
	testConfig.Server.TLS.AutoTLS = false
	testConfig.Server.TLS.CertPEM = certPEM
	testConfig.Server.TLS.KeyPEM = keyPEM
	testConfig.Server.TLS.HTTPRedirect = true
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)
	defer UnloadConfig()

	kernel := NewTestKernel(nil)
	server := NewServer(kernel)
	server.server.Addr = "-1"
	server.HTTPServer().Addr = "-1"

	startErr := server.Start()
	require.Error(t, startErr)
	require.Contains(t, startErr.Error(), "https server failed to listen and serve")

	// Successful Start with dual listeners and graceful shutdown
	validServer := NewServer(kernel)
	validServer.server.Addr = "127.0.0.1:0"
	validServer.HTTPServer().Addr = "127.0.0.1:0"
	serverErrorChannel := make(chan error, 1)
	go func() {
		serverErrorChannel <- validServer.Start()
	}()

	time.Sleep(30 * time.Millisecond)
	require.NoError(t, validServer.Shutdown(context.Background()))
	require.NoError(t, <-serverErrorChannel)
}
