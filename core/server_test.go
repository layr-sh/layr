package core

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

func TestCoreServerAllEndpointsUnit(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
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
	clientRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	clientResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientResponseRecorder, clientRequest)
	if clientResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with valid key, got %d", clientResponseRecorder.Code)
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
			inFlightRequest.Header.Set("X-Layr-Client-Publishable-Key", cryptoKeyManager.DerivePublishableKey())
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
	request.Header.Set("X-Layr-Client-Publishable-Key", cryptoKeyManager.DerivePublishableKey())
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
	userRequest.Header.Set("X-Refresh-Token", "test-refresh-token")
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
