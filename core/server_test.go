package core

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	server := NewServer(nil, cryptoKeyManager)
	if server.Mux() == nil {
		t.Fatal("expected non-nil Mux")
	}
	if server.Router() == nil {
		t.Fatal("expected non-nil Router")
	}
	if server.ControlPlaneRouter() == nil {
		t.Fatal("expected non-nil ControlPlaneRouter")
	}

	// Test /healthz
	healthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(healthRecorder, healthRequest)

	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", healthRecorder.Code)
	}

	var healthResponse map[string]any
	if err := json.Unmarshal(healthRecorder.Body.Bytes(), &healthResponse); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if healthResponse["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", healthResponse["status"])
	}

	// Test /readyz without DB pool (standalone check)
	readyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(readyRecorder, readyRequest)

	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected /readyz 200, got %d", readyRecorder.Code)
	}

	// Test /metrics
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsRecorder, metricsRequest)

	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("expected /metrics 200, got %d", metricsRecorder.Code)
	}
	metricsBody := metricsRecorder.Body.String()
	if !strings.Contains(metricsBody, "uptime_seconds") || !strings.Contains(metricsBody, "http_requests_total") {
		t.Fatalf("metrics body missing expected prometheus metrics: %s", metricsBody)
	}

	// Test /api/v1/topology without publishable key -> 200 (System Discovery is public & unauthenticated)
	topologyRequestNoKey := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/topology", nil)
	topologyRecorderNoKey := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(topologyRecorderNoKey, topologyRequestNoKey)
	if topologyRecorderNoKey.Code != http.StatusOK {
		t.Fatalf("expected topology 200 without key, got %d", topologyRecorderNoKey.Code)
	}

	// Register a public client test endpoint on router
	GetRoute[string](server.Router(), "/api/v1/client-test", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok-client"))
	})

	// Test public client endpoint without publishable key -> 401
	clientRequestNoKey := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/client-test", nil)
	clientRecorderNoKey := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientRecorderNoKey, clientRequestNoKey)
	if clientRecorderNoKey.Code != http.StatusUnauthorized {
		t.Fatalf("expected client endpoint 401 without key, got %d", clientRecorderNoKey.Code)
	}

	// Test public client endpoint with valid publishable key -> 200
	publishableKey := cryptoKeyManager.DerivePublishableKey()
	clientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/client-test", nil)
	clientRequest.Header.Set("X-Layr-Publishable-Key", publishableKey)
	clientRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientRecorder, clientRequest)
	if clientRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with valid key, got %d", clientRecorder.Code)
	}

	// Test public client endpoint with Service Account Key fallback -> 200
	clientServiceAccountRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/client-test", nil)
	clientServiceAccountRequest.Header.Set("X-Layr-Service-Account-Key", "sec_test_key_123456789")
	clientServiceAccountRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientServiceAccountRecorder, clientServiceAccountRequest)
	if clientServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with service account key fallback, got %d", clientServiceAccountRecorder.Code)
	}

	// Test public client endpoint with Authorization header fallback -> 200
	clientAuthHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/client-test", nil)
	clientAuthHeaderRequest.Header.Set("Authorization", "Bearer test_token")
	clientAuthHeaderRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(clientAuthHeaderRecorder, clientAuthHeaderRequest)
	if clientAuthHeaderRecorder.Code != http.StatusOK {
		t.Fatalf("expected client endpoint 200 with Authorization header fallback, got %d", clientAuthHeaderRecorder.Code)
	}

	// Test PublishableKeyMiddleware with nil KeyManager -> passes through
	nilKeyManagerSrv := NewServer(nil, nil)
	nilKeyManagerClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/topology", nil)
	nilKeyManagerClientRecorder := httptest.NewRecorder()
	nilKeyManagerSrv.server.Handler.ServeHTTP(nilKeyManagerClientRecorder, nilKeyManagerClientRequest)
	if nilKeyManagerClientRecorder.Code != http.StatusOK {
		t.Fatalf("expected topology 200 with nil key manager, got %d", nilKeyManagerClientRecorder.Code)
	}

	// Test Public OpenAPI /api/v1/spec.json
	jsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/spec.json", nil)
	jsonSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(jsonSpecRecorder, jsonSpecRequest)
	if jsonSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/spec.json 200, got %d", jsonSpecRecorder.Code)
	}
	if !strings.Contains(jsonSpecRecorder.Body.String(), "Layr Client API Engine") {
		t.Fatalf("expected public openapi spec in response: %s", jsonSpecRecorder.Body.String())
	}

	// Test Public OpenAPI /api/v1/spec.yaml
	yamlSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/spec.yaml", nil)
	yamlSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(yamlSpecRecorder, yamlSpecRequest)
	if yamlSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/spec.yaml 200, got %d", yamlSpecRecorder.Code)
	}
	if !strings.Contains(yamlSpecRecorder.Body.String(), "Layr Client API Engine") {
		t.Fatalf("expected public openapi 3.1.0 in yaml response: %s", yamlSpecRecorder.Body.String())
	}

	// Test Control Plane OpenAPI /api/v1/_/spec.json
	controlPlaneJsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/_/spec.json", nil)
	controlPlaneJsonSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaneJsonSpecRecorder, controlPlaneJsonSpecRequest)
	if controlPlaneJsonSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/_/spec.json 200, got %d", controlPlaneJsonSpecRecorder.Code)
	}
	if !strings.Contains(controlPlaneJsonSpecRecorder.Body.String(), "Layr Control Plane API Engine") {
		t.Fatalf("expected control plane openapi spec in response: %s", controlPlaneJsonSpecRecorder.Body.String())
	}

	// Test Control Plane OpenAPI /api/v1/_/spec.yaml
	controlPlaneYamlRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/_/spec.yaml", nil)
	controlPlaneYamlRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaneYamlRecorder, controlPlaneYamlRequest)
	if controlPlaneYamlRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/_/spec.yaml 200, got %d", controlPlaneYamlRecorder.Code)
	}
	if !strings.Contains(controlPlaneYamlRecorder.Body.String(), "Layr Control Plane API Engine") {
		t.Fatalf("expected control plane openapi 3.1.0 in yaml response: %s", controlPlaneYamlRecorder.Body.String())
	}
}

func TestCoreServerStartAndShutdownErrorsUnit(t *testing.T) {
	// 1. Test Start() error with invalid listen address
	invalidAddressServer := NewServer(nil, nil)
	invalidAddressServer.server.Addr = "invalid:address:too:many:colons"
	if err := invalidAddressServer.Start(); err == nil {
		t.Fatal("expected error from Start with invalid listen address, got nil")
	}

	// 2. Test Shutdown() error when context expires with in-flight connection
	drainServer := NewServer(nil, nil)
	handlerStartedChannel := make(chan struct{})
	unblockHandlerChannel := make(chan struct{})

	GetRoute[string](drainServer.Router(), "/api/v1/blocking-test", func(responseWriter http.ResponseWriter, request *http.Request) {
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
		inFlightRequest, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+tcpListener.Addr().String()+"/api/v1/blocking-test", nil)
		if requestErr == nil {
			response, clientErr := http.DefaultClient.Do(inFlightRequest)
			if clientErr == nil {
				_ = response.Body.Close()
			}
		}
	}()

	<-handlerStartedChannel

	// In-flight connection is active, shutdown with an already-canceled context
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()

	shutdownErr := drainServer.Shutdown(canceledContext)
	close(unblockHandlerChannel)

	if shutdownErr == nil {
		t.Fatal("expected error from Shutdown with canceled context on active connection, got nil")
	}
}
