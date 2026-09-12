package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/auth/jwt"
	"layr.sh/auth/oauth"
	"layr.sh/core"
)

func TestAuthHandlerOAuthLifecycleIntegration(t *testing.T) {
	db, cryptoKeyManager, teardown := setupTestDatabase(t)
	defer teardown()

	configManager := NewConfigManager(db, cryptoKeyManager)
	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	baseHandler.SetEventBus(eventBus)

	// Configure Google OAuth provider
	activeConfig := DefaultConfig()
	activeConfig.OAuthProviders["google"] = OAuthProviderConfig{
		Enabled:      true,
		ClientID:     "google-client-integration-id",
		ClientSecret: "google-client-integration-secret",
	}
	configManager.Set(activeConfig)

	// 1. Mock OAuth exchange HTTP client
	oauth.SetHTTPClient(&mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-access-token-1","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"oauth-user-777","email":"oauth777@gmail.com","name":"OAuth User","picture":"https://avatar.png"}`)),
			}, nil
		},
	})
	defer oauth.SetHTTPClient(nil)

	// 2. Authorize redirect generates state in KV store
	oauthAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize", nil)
	oauthAuthRequest.SetPathValue("provider", "google")
	oauthAuthResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(oauthAuthResponseResponseRecorder, oauthAuthRequest)
	if oauthAuthResponseResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on OAuth authorize, got: %d", oauthAuthResponseResponseRecorder.Code)
	}

	authLocation := oauthAuthResponseResponseRecorder.Header().Get("Location")
	var generatedState string
	parts := strings.Split(authLocation, "state=")
	if len(parts) > 1 {
		generatedState = strings.Split(parts[1], "&")[0]
	}
	if generatedState == "" {
		t.Fatalf("failed to extract state parameter from redirect URL: %s", authLocation)
	}

	// 3. Callback with valid state creates new user and links identity
	callbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("/api/v1/auth/oauth/google/callback?code=valid-mock-code&state=%s", generatedState), nil)
	callbackRequest.SetPathValue("provider", "google")
	callbackResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(callbackResponseResponseRecorder, callbackRequest)
	if callbackResponseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on first OAuth callback, got: %d (%s)", callbackResponseResponseRecorder.Code, callbackResponseResponseRecorder.Body.String())
	}

	var firstSessionResponse SessionResponse
	if err := json.Unmarshal(callbackResponseResponseRecorder.Body.Bytes(), &firstSessionResponse); err != nil {
		t.Fatalf("failed to decode auth response JSON: %v", err)
	}
	if firstSessionResponse.User.Email == nil || *firstSessionResponse.User.Email != "oauth777@gmail.com" {
		t.Fatalf("unexpected user email: %v", firstSessionResponse.User.Email)
	}
	if firstSessionResponse.User.IsAnonymous {
		t.Fatal("expected is_anonymous to be false for federated user")
	}

	// 4. Second callback with same identity logs in existing user
	_ = testKVStore.Set(context.Background(), "auth:pkce:re-login-state", "re-login-state", 10*time.Minute)
	secondCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=valid-mock-code&state=re-login-state", nil)
	secondCallbackRequest.SetPathValue("provider", "google")
	secondCallbackResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(secondCallbackResponseResponseRecorder, secondCallbackRequest)
	if secondCallbackResponseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second OAuth callback, got: %d", secondCallbackResponseResponseRecorder.Code)
	}

	var secondSessionResponse SessionResponse
	_ = json.Unmarshal(secondCallbackResponseResponseRecorder.Body.Bytes(), &secondSessionResponse)
	if secondSessionResponse.User.ID != firstSessionResponse.User.ID {
		t.Fatalf("expected same user ID %s on second login, got: %s", firstSessionResponse.User.ID, secondSessionResponse.User.ID)
	}

	// 5. UserInfo GET returns user from database
	userInfoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	userInfoRequest.Header.Set("Authorization", "Bearer "+secondSessionResponse.AccessToken)
	userInfoResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthUserInfo(userInfoResponseResponseRecorder, userInfoRequest)
	if userInfoResponseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on userinfo, got: %d", userInfoResponseResponseRecorder.Code)
	}

	// 5b. UserInfo 404 for non-existent user
	nonExistentUserID := uuid.NewV7().String()
	nonExistentToken, nonExistentTokenErr := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: nonExistentUserID,
		Role:    "authenticated",
	}, 900)
	if nonExistentTokenErr != nil {
		t.Fatalf("failed to generate access token for non-existent user: %v", nonExistentTokenErr)
	}
	nonExistentUserInfoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	nonExistentUserInfoRequest.Header.Set("Authorization", "Bearer "+nonExistentToken)
	nonExistentUserInfoResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthUserInfo(nonExistentUserInfoResponseRecorder, nonExistentUserInfoRequest)
	if nonExistentUserInfoResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent user info, got: %d", nonExistentUserInfoResponseRecorder.Code)
	}

	// 6. Anonymous user conversion
	// First, create an anonymous user in the database
	anonUserID := uuid.NewV7().String()
	_, insertErr := db.Exec(context.Background(), `
		INSERT INTO auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, '{"theme":"dark"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	if insertErr != nil {
		t.Fatalf("failed to insert anonymous user: %v", insertErr)
	}

	anonAccessToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token for anonymous user: %v", err)
	}

	// Test HandleOAuthAuthorize with active anonymous caller
	anonAuthorizeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize", nil)
	anonAuthorizeRequest.SetPathValue("provider", "google")
	anonAuthorizeRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	anonAuthorizeResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(anonAuthorizeResponseRecorder, anonAuthorizeRequest)
	if anonAuthorizeResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on anon oauth authorize, got: %d", anonAuthorizeResponseRecorder.Code)
	}

	// Mock a distinct user profile for anonymous conversion
	oauth.SetHTTPClient(&mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-anon-access-token","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"oauth-user-anon-converted","email":"converted@example.com","name":"Converted User"}`)),
			}, nil
		},
	})

	_ = testKVStore.Set(context.Background(), "auth:pkce:anon-conversion-state", "anon-conversion-state", 10*time.Minute)
	conversionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=anon-code&state=anon-conversion-state", nil)
	conversionRequest.SetPathValue("provider", "google")
	conversionRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	conversionResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(conversionResponseResponseRecorder, conversionRequest)
	if conversionResponseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on anonymous conversion, got: %d (%s)", conversionResponseResponseRecorder.Code, conversionResponseResponseRecorder.Body.String())
	}

	var convertedSessionResponse SessionResponse
	_ = json.Unmarshal(conversionResponseResponseRecorder.Body.Bytes(), &convertedSessionResponse)
	if convertedSessionResponse.User.ID != anonUserID {
		t.Fatalf("expected preserved anonymous user ID %s, got: %s", anonUserID, convertedSessionResponse.User.ID)
	}
	if convertedSessionResponse.User.IsAnonymous {
		t.Fatal("expected is_anonymous to be false after conversion")
	}
	if convertedSessionResponse.User.Email == nil || *convertedSessionResponse.User.Email != "converted@example.com" {
		t.Fatalf("unexpected converted email: %v", convertedSessionResponse.User.Email)
	}

	// 7. Conflict handling: identity conflict (another anonymous user tries to link already linked sub)
	anonUserID2 := uuid.NewV7().String()
	_, _ = db.Exec(context.Background(), `
		INSERT INTO auth.users (id, email, phone, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonUserID2)
	anonAccessToken2, _ := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID2,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	_ = testKVStore.Set(context.Background(), "auth:pkce:identity-conflict-state", "identity-conflict-state", 10*time.Minute)
	conflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=conflict-code&state=identity-conflict-state", nil)
	conflictRequest.SetPathValue("provider", "google")
	conflictRequest.Header.Set("Authorization", "Bearer "+anonAccessToken2)
	conflictResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(conflictResponseResponseRecorder, conflictRequest)
	if conflictResponseResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on identity already linked, got: %d", conflictResponseResponseRecorder.Code)
	}

	// 8. Conflict handling: email conflict (different sub, but email belongs to another account)
	oauth.SetHTTPClient(&mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-conflict-token","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"oauth-unique-sub-different","email":"converted@example.com","name":"Email Conflict"}`)),
			}, nil
		},
	})

	_ = testKVStore.Set(context.Background(), "auth:pkce:email-conflict-state", "email-conflict-state", 10*time.Minute)
	emailConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=conflict-email-code&state=email-conflict-state", nil)
	emailConflictRequest.SetPathValue("provider", "google")
	emailConflictRequest.Header.Set("Authorization", "Bearer "+anonAccessToken2)
	emailConflictResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(emailConflictResponseResponseRecorder, emailConflictRequest)
	if emailConflictResponseResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on email in use, got: %d", emailConflictResponseResponseRecorder.Code)
	}

	// 9. OIDC authorization state linkage inserts into sessions table
	oidcStateID := "oidc_integration_state_123"
	oidcPayloadJSON, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:      "client-integration-spa",
		RedirectURI:   "https://app.client.com/callback",
		Scope:         "openid profile",
		CodeChallenge: "challenge_integ_xyz",
		ClientState:   "client_state_integ",
	})
	_ = testKVStore.Set(context.Background(), "auth:oidc:state:"+oidcStateID, string(oidcPayloadJSON), 10*time.Minute)

	oauthStateWithOIDCJSON, _ := json.Marshal(OAuthStatePayload{
		StateID:     "oauth_oidc_state",
		Provider:    "google",
		OIDCStateID: oidcStateID,
	})
	_ = testKVStore.Set(context.Background(), "auth:pkce:oauth_oidc_state", string(oauthStateWithOIDCJSON), 10*time.Minute)

	oidcLinkRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=valid-code&state=oauth_oidc_state", nil)
	oidcLinkRequest.SetPathValue("provider", "google")
	oidcLinkResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(oidcLinkResponseResponseRecorder, oidcLinkRequest)
	if oidcLinkResponseResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found redirect on OIDC state linkage, got: %d", oidcLinkResponseResponseRecorder.Code)
	}

	// Verify session inserted in auth.sessions
	var sessionCount int
	_ = db.QueryRow(context.Background(), "SELECT count(*) FROM auth.sessions").Scan(&sessionCount)
	if sessionCount == 0 {
		t.Fatal("expected sessions to be inserted in database")
	}

	// 10. Broken pool coverage
	brokenDB := createBrokenPool(t)
	brokenBaseHandler := NewHandler(brokenDB, configManager, cryptoKeyManager)
	brokenBaseHandler.SetKVStore(testKVStore)

	_ = testKVStore.Set(context.Background(), "auth:pkce:broken-pool-state", "broken-pool-state", 10*time.Minute)
	brokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=valid-code&state=broken-pool-state", nil)
	brokenRequest.SetPathValue("provider", "google")
	brokenResponseRecorder := httptest.NewRecorder()
	brokenBaseHandler.HandleOAuthCallback(brokenResponseRecorder, brokenRequest)
	if brokenResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool, got: %d", brokenResponseRecorder.Code)
	}

	brokenUserInfoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	brokenUserInfoRequest.Header.Set("Authorization", "Bearer "+secondSessionResponse.AccessToken)
	brokenUserInfoResponseRecorder := httptest.NewRecorder()
	brokenBaseHandler.HandleOAuthUserInfo(brokenUserInfoResponseRecorder, brokenUserInfoRequest)
	if brokenUserInfoResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on userinfo with broken pool, got: %d", brokenUserInfoResponseRecorder.Code)
	}

	// 11. Custom claims procedure coverage
	testUserClaimsID := uuid.NewV7().String()
	if claims := baseHandler.resolveCustomClaims(context.Background(), testUserClaimsID); claims != nil {
		t.Fatalf("expected nil claims before function creation, got: %+v", claims)
	}
	if claims := baseHandler.resolveCustomClaims(context.Background(), ""); claims != nil {
		t.Fatalf("expected nil claims with empty userID, got: %+v", claims)
	}

	// Create procedure returning valid JSON
	_, _ = db.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION public.auth_claims(user_id uuid)
		RETURNS jsonb LANGUAGE sql AS $$
			SELECT '{"custom_org":"layr","tier":"enterprise"}'::jsonb;
		$$;
	`)
	claims := baseHandler.resolveCustomClaims(context.Background(), testUserClaimsID)
	if claims == nil || claims["custom_org"] != "layr" {
		t.Fatalf("expected valid custom claims from procedure, got: %+v", claims)
	}

	// Procedure returning null
	_, _ = db.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION public.auth_claims(user_id uuid)
		RETURNS jsonb LANGUAGE sql AS $$
			SELECT 'null'::jsonb;
		$$;
	`)
	if claims := baseHandler.resolveCustomClaims(context.Background(), testUserClaimsID); claims != nil {
		t.Fatalf("expected nil claims when procedure returns null, got: %+v", claims)
	}

	// Procedure returning invalid JSON (array instead of object)
	_, _ = db.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION public.auth_claims(user_id uuid)
		RETURNS jsonb LANGUAGE sql AS $$
			SELECT '[1, 2, 3]'::jsonb;
		$$;
	`)
	if claims := baseHandler.resolveCustomClaims(context.Background(), testUserClaimsID); claims != nil {
		t.Fatalf("expected nil claims when procedure returns invalid json for map, got: %+v", claims)
	}

	// 13. Authenticate user via database session
	refreshTokenRaw := "db-test-refresh-token-xyz"
	refreshHash := jwt.HashRefreshToken(refreshTokenRaw)
	_, _ = db.Exec(context.Background(), `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, $2, '127.0.0.1', 'test-agent', clock_timestamp() + interval '1 day', clock_timestamp())
	`, firstSessionResponse.User.ID, refreshHash)

	dbSessionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	dbSessionRequest.Header.Set("X-Refresh-Token", refreshTokenRaw)
	authedUserID, authErr := baseHandler.authenticateUser(dbSessionRequest)
	if authErr != nil || authedUserID != firstSessionResponse.User.ID {
		t.Fatalf("expected authenticated user %s, got %s (err: %v)", firstSessionResponse.User.ID, authedUserID, authErr)
	}

	// 14. resolveAnonymousCaller branches
	regularAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	regularAuthRequest.Header.Set("Authorization", "Bearer "+secondSessionResponse.AccessToken)
	if anonUserRecord, resolveAnonymousCallerErr := baseHandler.resolveAnonymousCaller(regularAuthRequest); anonUserRecord != nil || !errors.Is(resolveAnonymousCallerErr, ErrAnonymousSessionNotFound) {
		t.Fatalf("expected nil, ErrAnonymousSessionNotFound for regular user in resolveAnonymousCaller, got: %+v (err: %v)", anonUserRecord, resolveAnonymousCallerErr)
	}

	if anonUserRecord, resolveAnonymousCallerErr := brokenBaseHandler.resolveAnonymousCaller(regularAuthRequest); anonUserRecord != nil || resolveAnonymousCallerErr == nil || errors.Is(resolveAnonymousCallerErr, ErrAnonymousSessionNotFound) {
		t.Fatalf("expected nil, DB error for broken db in resolveAnonymousCaller, got: %+v (err: %v)", anonUserRecord, resolveAnonymousCallerErr)
	}

	// 15. Failed anonymous user conversion update error
	failAnonUserID := uuid.NewV7().String()
	_, _ = db.Exec(context.Background(), `
		INSERT INTO auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, '{}'::jsonb, clock_timestamp(), clock_timestamp())
	`, failAnonUserID)

	failAnonToken, _ := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     failAnonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	_, _ = db.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION public.test_fail_update() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced update error';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_update_trigger BEFORE UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION public.test_fail_update();
	`)

	oauth.SetHTTPClient(&mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-fail-token","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"fail-update-sub","email":"failupdate@example.com","name":"Fail Update"}`)),
			}, nil
		},
	})

	_ = testKVStore.Set(context.Background(), "auth:pkce:fail-update-state", "fail-update-state", 10*time.Minute)
	failUpdateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=anon-code&state=fail-update-state", nil)
	failUpdateRequest.SetPathValue("provider", "google")
	failUpdateRequest.Header.Set("Authorization", "Bearer "+failAnonToken)
	failUpdateResponseResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(failUpdateResponseResponseRecorder, failUpdateRequest)

	_, _ = db.Exec(context.Background(), `
		DROP TRIGGER IF EXISTS fail_update_trigger ON auth.users;
		DROP FUNCTION IF EXISTS public.test_fail_update();
	`)

	if failUpdateResponseResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failed user conversion update, got: %d", failUpdateResponseResponseRecorder.Code)
	}
}
