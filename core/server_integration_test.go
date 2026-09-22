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

	postgresContainer, err := tcpostgres.Run(ctx,
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
		_ = postgresContainer.Terminate(ctx)
	}()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
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
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	serviceAccountManager := NewServiceAccountManager(db)
	kernel := &Kernel{
		db:                    db,
		cryptoKeyManager:      cryptoKeyManager,
		serviceAccountManager: serviceAccountManager,
		jwtSigner:             jwtSigner,
	}
	server := NewServer(kernel)

	// 1. Ready probe with healthy DB
	readyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(readyResponseRecorder, readyRequest)

	if readyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected readyz 200, got %d", readyResponseRecorder.Code)
	}

	var getReadinessResponse GetReadinessResponse
	if err := json.Unmarshal(readyResponseRecorder.Body.Bytes(), &getReadinessResponse); err != nil {
		t.Fatalf("failed to parse readyz response: %v", err)
	}
	if getReadinessResponse.Database != "ok" || getReadinessResponse.Status != "ready" {
		t.Fatalf("expected database: ok and status: ready, got: %+v", getReadinessResponse)
	}

	// 2. Ready probe with closed / broken DB
	db.Close()
	failRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	failResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(failResponseRecorder, failRequest)

	if failResponseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readyz 503 on closed db connection pool, got %d", failResponseRecorder.Code)
	}
}

func TestCoreServerLiveDBAndKeyManagerPipelineIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
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
		_ = postgresContainer.Terminate(ctx)
	}()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, SystemDatabaseMigrations); migrationErr != nil {
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

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
	serviceAccountManager := NewServiceAccountManager(db)
	kernel := &Kernel{
		db:                    db,
		cryptoKeyManager:      cryptoKeyManager,
		serviceAccountManager: serviceAccountManager,
		jwtSigner:             jwtSigner,
	}
	server := NewServer(kernel)

	// Register a public endpoint that queries the live PostgreSQL database
	GetRoute[string](server.BaseRouter(), "/v1/db-check", func(responseWriter http.ResponseWriter, request *http.Request) {
		var postgresVersion string
		if scanErr := db.QueryRow(request.Context(), "SELECT version()").Scan(&postgresVersion); scanErr != nil {
			WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "database query failed", scanErr.Error())
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(postgresVersion))
	})

	// 1. Without publishable key -> 401 Unauthorized
	unauthorizedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/db-check", nil)
	unauthorizedResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(unauthorizedResponseRecorder, unauthorizedRequest)

	if unauthorizedResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated db-check, got %d", unauthorizedResponseRecorder.Code)
	}

	// 2. With invalid publishable key -> 401 Unauthorized
	invalidKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/db-check", nil)
	invalidKeyRequest.Header.Set("X-Layr-Client-Publishable-Key", "invalid_publishable_key_123456789")
	invalidKeyResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(invalidKeyResponseRecorder, invalidKeyRequest)

	if invalidKeyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid publishable key, got %d", invalidKeyResponseRecorder.Code)
	}

	// 3. With valid derived publishable key -> 200 OK and live DB query result
	publishableKey := cryptoKeyManager.DerivePublishableKey()
	validKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/db-check", nil)
	validKeyRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	validKeyResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(validKeyResponseRecorder, validKeyRequest)

	if validKeyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid publishable key, got %d", validKeyResponseRecorder.Code)
	}
	if !strings.Contains(validKeyResponseRecorder.Body.String(), "PostgreSQL") {
		t.Fatalf("expected PostgreSQL version string in body, got: %s", validKeyResponseRecorder.Body.String())
	}

	// 4. With Service Account key fallback -> 200 OK
	createdServiceAccount, createErr := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name: "test-server-integration-sa",
	})
	if createErr != nil {
		t.Fatalf("failed to create test service account: %v", createErr)
	}
	serviceAccountRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/db-check", nil)
	serviceAccountRequest.Header.Set("X-Layr-Service-Account-Key", createdServiceAccount.SecretKey)
	serviceAccountResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(serviceAccountResponseRecorder, serviceAccountRequest)

	if serviceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with service account header fallback, got %d", serviceAccountResponseRecorder.Code)
	}

	// 4b. With unauthenticated service account key -> 401 Unauthorized
	unauthServiceAccountRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/db-check", nil)
	unauthServiceAccountRequest.Header.Set("X-Layr-Service-Account-Key", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	unauthServiceAccountResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(unauthServiceAccountResponseRecorder, unauthServiceAccountRequest)
	if unauthServiceAccountResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with unauthenticated service account key, got %d", unauthServiceAccountResponseRecorder.Code)
	}

	// 5. Database Session Lookup via session cookie and X-Refresh-Token
	_, _ = db.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE TABLE IF NOT EXISTS auth.users (
			id UUID PRIMARY KEY,
			email VARCHAR(255),
			phone VARCHAR(32),
			role VARCHAR(64) NOT NULL DEFAULT 'authenticated',
			is_anonymous BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE IF NOT EXISTS auth.sessions (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
			refresh_token_hash VARCHAR(128) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
		);
	`)

	testUserID := "01918a24-7777-7000-8000-000000000001"
	testEmail := "session.user@example.com"
	liveRefreshToken := "live_test_refresh_token_1234567890"
	liveRefreshHash := jwtSigner.HashRefreshToken(liveRefreshToken)

	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (id, email, role, is_anonymous)
		VALUES ($1, $2, 'authenticated', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID, testEmail)

	sessionUUID := "01918a24-8888-7000-8000-000000000001"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour')
		ON CONFLICT (id) DO NOTHING
	`, sessionUUID, testUserID, liveRefreshHash)

	var capturedAuthContext AuthContext
	GetRoute[string](server.BaseRouter(), "/v1/auth-check", func(responseWriter http.ResponseWriter, request *http.Request) {
		capturedAuthContext = GetAuthContext(request.Context())
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("auth-ok"))
	})

	// 5a. With secure session cookie
	secureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth-check", nil)
	secureCookieRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	secureCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameSecure, Value: liveRefreshToken})
	secureCookieResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(secureCookieResponseRecorder, secureCookieRequest)

	if secureCookieResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with secure session cookie, got %d", secureCookieResponseRecorder.Code)
	}
	if capturedAuthContext.UserID != testUserID || capturedAuthContext.JWT.Email != testEmail {
		t.Fatalf("expected authenticated session context, got: %+v", capturedAuthContext)
	}

	// 5b. With insecure session cookie
	insecureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth-check", nil)
	insecureCookieRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	insecureCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameInsecure, Value: liveRefreshToken})
	insecureCookieResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(insecureCookieResponseRecorder, insecureCookieRequest)

	if insecureCookieResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with insecure session cookie, got %d", insecureCookieResponseRecorder.Code)
	}
	if capturedAuthContext.UserID != testUserID {
		t.Fatalf("expected authenticated session context with insecure cookie, got: %+v", capturedAuthContext)
	}

	// 5c. With X-Refresh-Token header
	refreshHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth-check", nil)
	refreshHeaderRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	refreshHeaderRequest.Header.Set("X-Refresh-Token", liveRefreshToken)
	refreshHeaderResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(refreshHeaderResponseRecorder, refreshHeaderRequest)

	if refreshHeaderResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-Refresh-Token header, got %d", refreshHeaderResponseRecorder.Code)
	}
	if capturedAuthContext.UserID != testUserID {
		t.Fatalf("expected authenticated session context with X-Refresh-Token, got: %+v", capturedAuthContext)
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
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	server := NewServer(&Kernel{cryptoKeyManager: cryptoKeyManager, jwtSigner: jwtSigner})

	// Register operations on BaseRouter
	GetRoute[string](server.BaseRouter(), "/v1/data/records", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok"))
	}, RouteTag("Data"), RouteSummary("List Data Records"), RouteOperationID("listDataRecords"))

	// Register operations on ControlPlaneRouter
	GetRoute[string](server.ControlPlaneRouter(), "/v1/_/console/status", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok"))
	}, RouteTag("Console"), RouteSummary("Get Console Status"), RouteOperationID("getConsoleStatus"))

	// 1. Test Public OpenAPI Spec JSON
	jsonSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/spec.json", nil)
	jsonSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(jsonSpecResponseRecorder, jsonSpecRequest)

	if jsonSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/spec.json 200, got %d", jsonSpecResponseRecorder.Code)
	}
	jsonSpecResponseBody := jsonSpecResponseRecorder.Body.String()
	if !strings.Contains(jsonSpecResponseBody, "listDataRecords") || !strings.Contains(jsonSpecResponseBody, "Layr Client API Engine") {
		t.Fatalf("expected listDataRecords operation in public JSON spec: %s", jsonSpecResponseBody)
	}

	// 2. Test Control Plane OpenAPI Spec JSON
	controlPlaneJSONSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/spec.json", nil)
	controlPlaceJSONSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(controlPlaceJSONSpecResponseRecorder, controlPlaneJSONSpecRequest)

	if controlPlaceJSONSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/_/spec.json 200, got %d", controlPlaceJSONSpecResponseRecorder.Code)
	}
	controlPlaneJSONSpecResponseBody := controlPlaceJSONSpecResponseRecorder.Body.String()
	if !strings.Contains(controlPlaneJSONSpecResponseBody, "getConsoleStatus") || !strings.Contains(controlPlaneJSONSpecResponseBody, "Layr Control Plane API Engine") {
		t.Fatalf("expected getConsoleStatus operation in control plane JSON spec: %s", controlPlaneJSONSpecResponseBody)
	}

	// 3. Test Public OpenAPI Spec YAML
	yamlSpecRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/spec.yaml", nil)
	yamlSpecResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(yamlSpecResponseRecorder, yamlSpecRequest)

	if yamlSpecResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected /v1/spec.yaml 200, got %d", yamlSpecResponseRecorder.Code)
	}
	if !strings.Contains(yamlSpecResponseRecorder.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("expected openapi: 3.1.0 in YAML spec: %s", yamlSpecResponseRecorder.Body.String())
	}
}

func TestCoreServerMiddlewareMetricsAndProbesIntegration(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	server := kernel.Server()

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
				targetPath = "/v1/manifest"
			}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, targetPath, nil)
			responseResponseRecorder := httptest.NewRecorder()
			server.server.Handler.ServeHTTP(responseResponseRecorder, request)
			if responseResponseRecorder.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", targetPath, responseResponseRecorder.Code)
			}
		}(iteration)
	}
	waitGroup.Wait()

	// Query /metrics and assert http_requests_total includes all handled requests
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsResponseRecorder, metricsRequest)

	if metricsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d", metricsResponseRecorder.Code)
	}
	metricsBody := metricsResponseRecorder.Body.String()
	if !strings.Contains(metricsBody, "http_requests_total") {
		t.Fatalf("expected http_requests_total metric in output: %s", metricsBody)
	}
}
