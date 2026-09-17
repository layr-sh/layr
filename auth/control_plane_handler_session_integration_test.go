package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerSessionIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	kvStore := newInMemoryKVStore()

	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Auth Control Plane Service Account",
		Scopes: []string{"*"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	configManager := NewConfigManager(db, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager)
	controlPlaneHandler.SetKVStore(kvStore)
	controlPlaneHandler.SetEventBus(eventBus)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)

	authBearerHeader := "Bearer " + createdServiceAccount.SecretKey

	// Create test user directly in DB
	testUserID := uuid.NewV7().String()
	_, err = db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, 'session-test@example.com', 'hash', 'authenticated', '{}'::jsonb, clock_timestamp(), clock_timestamp())
	`, testUserID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Insert session for user
	_, err = db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, 'hash_abc', '127.0.0.1', 'Mozilla/5.0', clock_timestamp() + interval '30 days', clock_timestamp())
	`, testUserID)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	// 1. List user sessions
	sessionsListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+testUserID+"/sessions", nil)
	sessionsListRequest.Header.Set("Authorization", authBearerHeader)
	sessionsListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUserSessions(sessionsListResponseRecorder, sessionsListRequest)

	if sessionsListResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleListUserSessions, got: %d", sessionsListResponseRecorder.Code)
	}
	if !strings.Contains(sessionsListResponseRecorder.Body.String(), `"count":1`) {
		t.Fatalf("expected session count 1, got: %s", sessionsListResponseRecorder.Body.String())
	}

	// 2. Revoke user sessions
	revokeSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+testUserID+"/sessions/revoke", nil)
	revokeSessionsRequest.Header.Set("Authorization", authBearerHeader)
	revokeSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(revokeSessionsResponseRecorder, revokeSessionsRequest)

	if revokeSessionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleRevokeUserSessions, got: %d", revokeSessionsResponseRecorder.Code)
	}

	// Verify sessions are gone
	var count int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM auth.sessions WHERE user_id = $1", testUserID).Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 sessions after revocation, got: %d", count)
	}
}

func TestAuthControlPlaneHandlerSessionBrokenPoolIntegration(t *testing.T) {
	brokenDB := createBrokenPool(t)
	if brokenDB == nil {
		t.Skip("skipping broken pool test")
		return
	}

	cryptoKeyManager, _ := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	configManager := NewConfigManager(brokenDB, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(brokenDB, configManager)
	randomID := uuid.NewV7().String()

	ctx := context.Background()

	// 1. List Sessions error
	listSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+randomID+"/sessions", nil)
	listSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUserSessions(listSessionsResponseRecorder, listSessionsRequest)
	if listSessionsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool list sessions, got: %d", listSessionsResponseRecorder.Code)
	}

	// 2. Revoke Sessions error
	revokeSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+randomID+"/sessions/revoke", nil)
	revokeSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(revokeSessionsResponseRecorder, revokeSessionsRequest)
	if revokeSessionsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool revoke sessions, got: %d", revokeSessionsResponseRecorder.Code)
	}
}

func TestAuthControlPlaneRevokeUserSessionsBackChannelIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	kvStore := newInMemoryKVStore()

	createdServiceAccount, serviceAccountErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Auth Control Plane Revoke Service Account",
		Scopes: []string{"*"},
	})
	if serviceAccountErr != nil {
		t.Fatalf("failed to create service account: %v", serviceAccountErr)
	}

	var receivedTokenMutex sync.Mutex
	var receivedSignOutToken string
	mockRPServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		readBytes, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}
		parsedValues, parseErr := url.ParseQuery(string(readBytes))
		if parseErr != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedTokenMutex.Lock()
		receivedSignOutToken = parsedValues.Get("logout_token")
		receivedTokenMutex.Unlock()
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mockRPServer.Close()

	configManager := NewConfigManager(db, cryptoKeyManager)
	authConfig := DefaultConfig()
	authConfig.OIDC.Enabled = true
	authConfig.OIDC.Clients = []OIDCClientConfig{
		{
			ClientID:                           "client-federated-rp",
			Name:                               "Federated Relying Party",
			BackChannelSignOutURI:              mockRPServer.URL,
			BackChannelSignOutSessionRequired:  true,
			FrontChannelSignOutURI:             "",
			FrontChannelSignOutSessionRequired: false,
		},
	}
	configManager.Set(authConfig)

	jwtSigner, signerErr := core.NewJWTSigner(cryptoKeyManager)
	if signerErr != nil {
		t.Fatalf("failed to create jwt signer: %v", signerErr)
	}

	controlPlaneHandler := NewControlPlaneHandler(db, configManager)
	controlPlaneHandler.SetKVStore(kvStore)
	controlPlaneHandler.SetEventBus(eventBus)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	controlPlaneHandler.SetJWTSigner(jwtSigner)
	controlPlaneHandler.SetHTTPClient(mockRPServer.Client())

	authBearerHeader := "Bearer " + createdServiceAccount.SecretKey

	testUserID := uuid.NewV7().String()
	_, insertUserErr := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, 'federated-revoke@example.com', 'hash', 'authenticated', '{}'::jsonb, clock_timestamp(), clock_timestamp())
	`, testUserID)
	if insertUserErr != nil {
		t.Fatalf("failed to create user: %v", insertUserErr)
	}

	targetClientID := "client-federated-rp"
	_, insertSessionErr := db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, client_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, $2, 'hash_federated_xyz', '127.0.0.1', 'Mozilla/5.0', clock_timestamp() + interval '30 days', clock_timestamp())
	`, testUserID, targetClientID)
	if insertSessionErr != nil {
		t.Fatalf("failed to insert session: %v", insertSessionErr)
	}

	revokeSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+testUserID+"/sessions/revoke", nil)
	revokeSessionsRequest.Header.Set("Authorization", authBearerHeader)
	revokeSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(revokeSessionsResponseRecorder, revokeSessionsRequest)

	if revokeSessionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleRevokeUserSessions, got: %d (%s)", revokeSessionsResponseRecorder.Code, revokeSessionsResponseRecorder.Body.String())
	}

	var sessionCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM auth.sessions WHERE user_id = $1", testUserID).Scan(&sessionCount)
	if sessionCount != 0 {
		t.Fatalf("expected 0 sessions after revocation, got: %d", sessionCount)
	}

	receivedTokenMutex.Lock()
	capturedToken := receivedSignOutToken
	receivedTokenMutex.Unlock()

	if capturedToken == "" {
		t.Fatal("expected mock RP to receive back-channel sign-out request with logout_token")
	}

	verifiedSignOutJWTClaims, verifyErr := jwtSigner.VerifySignOutToken(capturedToken)
	if verifyErr != nil {
		t.Fatalf("failed to verify received sign-out token: %v", verifyErr)
	}
	if verifiedSignOutJWTClaims.Subject != testUserID {
		t.Fatalf("expected subject %s, got: %s", testUserID, verifiedSignOutJWTClaims.Subject)
	}
	if verifiedSignOutJWTClaims.Audience != "client-federated-rp" {
		t.Fatalf("expected audience client-federated-rp, got: %s", verifiedSignOutJWTClaims.Audience)
	}
	if verifiedSignOutJWTClaims.Events[core.SignOutTokenEventURI] == nil {
		t.Fatalf("missing backchannel logout event claim in verified token")
	}
}
