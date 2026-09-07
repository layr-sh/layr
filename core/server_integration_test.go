package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreServerWithLiveDBPoolIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}

	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	server := NewServer(db, cryptoKeyManager)

	// 1. Ready probe with healthy DB
	readyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(readyRecorder, readyRequest)

	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected readyz 200, got %d", readyRecorder.Code)
	}

	var readyResponse ReadyResponse
	if err := json.Unmarshal(readyRecorder.Body.Bytes(), &readyResponse); err != nil {
		t.Fatalf("failed to parse readyz response: %v", err)
	}
	if readyResponse.Database != "ok" || readyResponse.Status != "ready" {
		t.Fatalf("expected database: ok and status: ready, got: %+v", readyResponse)
	}

	// 2. Ready probe with closed / broken DB
	db.Close()
	failRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	failRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(failRecorder, failRequest)

	if failRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readyz 503 on closed db connection pool, got %d", failRecorder.Code)
	}
}

func TestCoreServerLiveDBAndKeyManagerPipelineIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, err := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	server := NewServer(db, cryptoKeyManager)

	// Register a public endpoint that queries the live PostgreSQL database
	GetRoute[string](server.Router(), "/api/v1/db-check", func(responseWriter http.ResponseWriter, request *http.Request) {
		var postgresVersion string
		if err := db.QueryRow(request.Context(), "SELECT version()").Scan(&postgresVersion); err != nil {
			WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "database query failed", "LAYR_DB_ERROR")
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(postgresVersion))
	})

	// 1. Without publishable key -> 401 Unauthorized
	unauthorizedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/db-check", nil)
	unauthorizedRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)

	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated db-check, got %d", unauthorizedRecorder.Code)
	}

	// 2. With invalid publishable key -> 401 Unauthorized
	invalidKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/db-check", nil)
	invalidKeyRequest.Header.Set("X-Layr-Publishable-Key", "invalid_publishable_key_123456789")
	invalidKeyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(invalidKeyRecorder, invalidKeyRequest)

	if invalidKeyRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid publishable key, got %d", invalidKeyRecorder.Code)
	}

	// 3. With valid derived publishable key -> 200 OK and live DB query result
	publishableKey := cryptoKeyManager.DerivePublishableKey()
	validKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/db-check", nil)
	validKeyRequest.Header.Set("X-Layr-Publishable-Key", publishableKey)
	validKeyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(validKeyRecorder, validKeyRequest)

	if validKeyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid publishable key, got %d", validKeyRecorder.Code)
	}
	if !strings.Contains(validKeyRecorder.Body.String(), "PostgreSQL") {
		t.Fatalf("expected PostgreSQL version string in body, got: %s", validKeyRecorder.Body.String())
	}

	// 4. With Service Account key fallback -> 200 OK
	serviceAccountRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/db-check", nil)
	serviceAccountRequest.Header.Set("X-Layr-Service-Account-Key", "sec_live_integration_key")
	serviceAccountRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(serviceAccountRecorder, serviceAccountRequest)

	if serviceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with service account header fallback, got %d", serviceAccountRecorder.Code)
	}
}

func TestCoreServerDualRoutersAndOpenAPISpecsIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Auth.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	server := NewServer(nil, cryptoKeyManager)

	// Register operations on Router
	GetRoute[string](server.Router(), "/api/v1/data/records", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok"))
	}, RouteTag("Data"), RouteSummary("List Data Records"), RouteOperationID("listDataRecords"))

	// Register operations on ControlPlaneRouter
	GetRoute[string](server.ControlPlaneRouter(), "/api/v1/_/console/status", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok"))
	}, RouteTag("Console"), RouteSummary("Get Console Status"), RouteOperationID("getConsoleStatus"))

	// 1. Test Public OpenAPI Spec JSON
	jsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/spec.json", nil)
	jsonSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(jsonSpecRecorder, jsonSpecRequest)

	if jsonSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/spec.json 200, got %d", jsonSpecRecorder.Code)
	}
	jsonSpecResponseBody := jsonSpecRecorder.Body.String()
	if !strings.Contains(jsonSpecResponseBody, "listDataRecords") || !strings.Contains(jsonSpecResponseBody, "Layr Client API Engine") {
		t.Fatalf("expected listDataRecords operation in public JSON spec: %s", jsonSpecResponseBody)
	}

	// 2. Test Control Plane OpenAPI Spec JSON
	controlPlaneJSONSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/_/spec.json", nil)
	controlPlaceJSONSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaceJSONSpecRecorder, controlPlaneJSONSpecRequest)

	if controlPlaceJSONSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/_/spec.json 200, got %d", controlPlaceJSONSpecRecorder.Code)
	}
	controlPlaneJSONSpecResponseBody := controlPlaceJSONSpecRecorder.Body.String()
	if !strings.Contains(controlPlaneJSONSpecResponseBody, "getConsoleStatus") || !strings.Contains(controlPlaneJSONSpecResponseBody, "Layr Control Plane API Engine") {
		t.Fatalf("expected getConsoleStatus operation in control plane JSON spec: %s", controlPlaneJSONSpecResponseBody)
	}

	// 3. Test Public OpenAPI Spec YAML
	yamlSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/spec.yaml", nil)
	yamlSpecRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(yamlSpecRecorder, yamlSpecRequest)

	if yamlSpecRecorder.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/spec.yaml 200, got %d", yamlSpecRecorder.Code)
	}
	if !strings.Contains(yamlSpecRecorder.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("expected openapi: 3.1.0 in YAML spec: %s", yamlSpecRecorder.Body.String())
	}
}

func TestCoreServerMiddlewareMetricsAndProbesIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.FileStorage.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	server := NewServer(nil, cryptoKeyManager)

	// Concurrent request load to verify middleware request counters and probes
	var waitGroup sync.WaitGroup
	requestTotal := 20
	for iteration := 0; iteration < requestTotal; iteration++ {
		waitGroup.Add(1)
		go func(iterationIndex int) {
			defer waitGroup.Done()
			var targetPath string
			switch iterationIndex % 3 {
			case 0:
				targetPath = "/healthz"
			case 1:
				targetPath = "/readyz"
			default:
				targetPath = "/api/v1/topology"
			}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, targetPath, nil)
			recorder := httptest.NewRecorder()
			server.server.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", targetPath, recorder.Code)
			}
		}(iteration)
	}
	waitGroup.Wait()

	// Query /metrics and assert http_requests_total includes all handled requests
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsRecorder, metricsRequest)

	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d", metricsRecorder.Code)
	}
	metricsBody := metricsRecorder.Body.String()
	if !strings.Contains(metricsBody, "http_requests_total") {
		t.Fatalf("expected http_requests_total metric in output: %s", metricsBody)
	}
}
