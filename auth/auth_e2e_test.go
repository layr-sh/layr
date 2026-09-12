package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthAppUserLifecycleE2E(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	layrConfig := core.DefaultConfig()
	layrConfig.Auth.Enabled = true
	core.SetLoadedConfig(layrConfig)
	defer core.UnloadConfig()

	service := NewService(db, cryptoKeyManager)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}
	defer func() { _ = service.Stop() }()

	coreServer := core.NewServer(db, cryptoKeyManager)
	service.RegisterRoutes(coreServer.Router(), coreServer.ControlPlaneRouter())
	publishableKey := cryptoKeyManager.DerivePublishableKey()

	userEmail := "e2e-app-user@example.com"
	userPassword := "StrongSecurePassword123!#"

	// 1. User Sign Up
	signupPayload, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": userPassword,
		"properties": map[string]any{
			"tier": "enterprise",
		},
	})
	signupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", bytes.NewReader(signupPayload))
	signupRequest.Header.Set("Content-Type", "application/json")
	signupRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signupResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signupResponseRecorder, signupRequest)

	if signupResponseRecorder.Code != http.StatusCreated && signupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected signup 200/201, got %d (body: %s)", signupResponseRecorder.Code, signupResponseRecorder.Body.String())
	}

	var signupSessionResponse SessionResponse
	if err := json.NewDecoder(signupResponseRecorder.Body).Decode(&signupSessionResponse); err != nil {
		t.Fatalf("failed to decode signup response: %v", err)
	}
	if signupSessionResponse.User.ID == "" || signupSessionResponse.AccessToken == "" {
		t.Fatalf("invalid signup payload response: %+v", signupSessionResponse)
	}

	// 2. User Sign In
	signinPayload, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": userPassword,
	})
	signinRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signinPayload))
	signinRequest.Header.Set("Content-Type", "application/json")
	signinRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signinResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signinResponseRecorder, signinRequest)

	if signinResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected login 200 OK, got %d (body: %s)", signinResponseRecorder.Code, signinResponseRecorder.Body.String())
	}

	var signinSessionResponse SessionResponse
	if err := json.NewDecoder(signinResponseRecorder.Body).Decode(&signinSessionResponse); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// 3. Authenticated Profile Access via User Export
	exportRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users/"+signupSessionResponse.User.ID+"/export", nil)
	exportRequest.Header.Set("Authorization", "Bearer "+signinSessionResponse.AccessToken)
	exportResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(exportResponseRecorder, exportRequest)

	if exportResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected user export 200 OK, got %d (body: %s)", exportResponseRecorder.Code, exportResponseRecorder.Body.String())
	}

	// 4. Token Refresh
	refreshPayload, _ := json.Marshal(map[string]any{
		"refresh_token": signinSessionResponse.RefreshToken,
	})
	refreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(refreshPayload))
	refreshRequest.Header.Set("Content-Type", "application/json")
	refreshRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	refreshResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(refreshResponseRecorder, refreshRequest)

	if refreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected refresh 200 OK, got %d (body: %s)", refreshResponseRecorder.Code, refreshResponseRecorder.Body.String())
	}

	var refreshedSessionResponse SessionResponse
	_ = json.NewDecoder(refreshResponseRecorder.Body).Decode(&refreshedSessionResponse)

	// 5. Sign Out / Session Revocation
	signoutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-out", nil)
	signoutRequest.Header.Set("Authorization", "Bearer "+refreshedSessionResponse.AccessToken)
	signoutRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: refreshedSessionResponse.RefreshToken})
	signoutResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signoutResponseRecorder, signoutRequest)

	if signoutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout 200 OK, got %d (body: %s)", signoutResponseRecorder.Code, signoutResponseRecorder.Body.String())
	}

	// 6. Old Refresh Token Must Now Fail
	revokedRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(refreshPayload))
	revokedRefreshRequest.Header.Set("Content-Type", "application/json")
	revokedRefreshRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	revokedRefreshResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(revokedRefreshResponseRecorder, revokedRefreshRequest)

	if revokedRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for revoked refresh token, got %d", revokedRefreshResponseRecorder.Code)
	}
}

func TestAuthSelfServiceSessionsE2E(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	layrConfig := core.DefaultConfig()
	layrConfig.Auth.Enabled = true
	core.SetLoadedConfig(layrConfig)
	defer core.UnloadConfig()

	service := NewService(db, cryptoKeyManager)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}
	defer func() { _ = service.Stop() }()

	coreServer := core.NewServer(db, cryptoKeyManager)
	service.RegisterRoutes(coreServer.Router(), coreServer.ControlPlaneRouter())
	publishableKey := cryptoKeyManager.DerivePublishableKey()

	userEmail := "e2e-sessions-user@example.com"
	userPassword := "SessionStrongPassword123!"

	// 1. Sign up user
	signupPayload, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": userPassword,
	})
	signupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", bytes.NewReader(signupPayload))
	signupRequest.Header.Set("Content-Type", "application/json")
	signupRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signupResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signupResponseRecorder, signupRequest)
	if signupResponseRecorder.Code != http.StatusOK && signupResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 on signup: %d", signupResponseRecorder.Code)
	}
	var firstSessionResponse SessionResponse
	_ = json.NewDecoder(signupResponseRecorder.Body).Decode(&firstSessionResponse)

	// 2. Sign in from second device
	signinPayload, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": userPassword,
	})
	signinRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signinPayload))
	signinRequest.Header.Set("Content-Type", "application/json")
	signinRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signinRequest.Header.Set("User-Agent", "Secondary-Device-Tablet")
	signinResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signinResponseRecorder, signinRequest)
	if signinResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on signin: %d", signinResponseRecorder.Code)
	}
	var secondSessionResponse SessionResponse
	_ = json.NewDecoder(signinResponseRecorder.Body).Decode(&secondSessionResponse)

	// 3. List active sessions via E2E server mux
	listSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/sessions", nil)
	listSessionsRequest.Header.Set("Authorization", "Bearer "+firstSessionResponse.AccessToken)
	listSessionsRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: firstSessionResponse.RefreshToken})
	listSessionsResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(listSessionsResponseRecorder, listSessionsRequest)
	if listSessionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list sessions: %d", listSessionsResponseRecorder.Code)
	}
	var listUserSessionsResponse ListUserSessionsResponse
	_ = json.NewDecoder(listSessionsResponseRecorder.Body).Decode(&listUserSessionsResponse)
	if listUserSessionsResponse.Count != 2 {
		t.Fatalf("expected 2 sessions, got: %d", listUserSessionsResponse.Count)
	}

	var secondarySessionID string
	for _, sessionRecord := range listUserSessionsResponse.Sessions {
		if sessionRecord.UserAgent != nil && *sessionRecord.UserAgent == "Secondary-Device-Tablet" {
			secondarySessionID = sessionRecord.ID
		}
	}
	if secondarySessionID == "" {
		t.Fatalf("expected to find secondary device session ID")
	}

	// 4. Revoke secondary session via E2E server mux
	deleteSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/sessions/"+secondarySessionID, nil)
	deleteSessionRequest.Header.Set("Authorization", "Bearer "+firstSessionResponse.AccessToken)
	deleteSessionResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(deleteSessionResponseRecorder, deleteSessionRequest)
	if deleteSessionResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete session: %d", deleteSessionResponseRecorder.Code)
	}

	// 5. Revoke other sessions via E2E server mux (now only 1 session left)
	revokeOthersRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sessions/revoke-others", nil)
	revokeOthersRequest.Header.Set("Authorization", "Bearer "+firstSessionResponse.AccessToken)
	revokeOthersRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: firstSessionResponse.RefreshToken})
	revokeOthersResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(revokeOthersResponseRecorder, revokeOthersRequest)
	if revokeOthersResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on revoke others: %d", revokeOthersResponseRecorder.Code)
	}
}

func TestAuthUserSelfServiceLifecycleE2E(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	layrConfig := core.DefaultConfig()
	layrConfig.Auth.Enabled = true
	core.SetLoadedConfig(layrConfig)
	defer core.UnloadConfig()

	service := NewService(db, cryptoKeyManager)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}
	defer func() { _ = service.Stop() }()

	activeConfig := service.configManager.Get()
	activeConfig.Anonymous.Enabled = true
	service.configManager.Set(activeConfig)

	coreServer := core.NewServer(db, cryptoKeyManager)
	service.RegisterRoutes(coreServer.Router(), coreServer.ControlPlaneRouter())
	publishableKey := cryptoKeyManager.DerivePublishableKey()

	// 1. Anonymous User Setup
	anonUserID := "01918a24-7777-7000-8000-000000000007"
	_, err := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, '{}', clock_timestamp(), clock_timestamp())
	`, anonUserID)
	if err != nil {
		t.Fatalf("failed to insert anonymous test user: %v", err)
	}

	currentAccessToken, err := service.handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// 2. Inspect Anonymous Profile
	initialProfileRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/user", nil)
	initialProfileRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	initialProfileResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(initialProfileResponseRecorder, initialProfileRequest)
	if initialProfileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from profile, got: %d (%s)", initialProfileResponseRecorder.Code, initialProfileResponseRecorder.Body.String())
	}
	var initialUserResponse UserResponse
	_ = json.NewDecoder(initialProfileResponseRecorder.Body).Decode(&initialUserResponse)
	if !initialUserResponse.IsAnonymous {
		t.Fatalf("expected is_anonymous to be true")
	}

	// 3. Attempting to set password on anonymous user MUST FAIL (400 Bad Request LAYR_AUTH_001)
	forbiddenPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		NewPassword: "AttemptedPassword123!",
	})
	forbiddenPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(forbiddenPasswordPayload))
	forbiddenPasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	forbiddenPasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(forbiddenPasswordResponseRecorder, forbiddenPasswordRequest)
	if forbiddenPasswordResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(forbiddenPasswordResponseRecorder.Body.String(), "LAYR_AUTH_001") {
		t.Fatalf("expected 400 LAYR_AUTH_001 on anonymous password change, got: %d (%s)", forbiddenPasswordResponseRecorder.Code, forbiddenPasswordResponseRecorder.Body.String())
	}

	// 4. Update Profile Properties
	patchPropertiesPayload, _ := json.Marshal(UpdateUserPropertiesRequest{
		Properties: map[string]any{
			"display_name": "E2E Ghost",
			"theme":        "dark",
		},
	})
	patchPropertiesRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/properties", bytes.NewReader(patchPropertiesPayload))
	patchPropertiesRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	patchPropertiesResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(patchPropertiesResponseRecorder, patchPropertiesRequest)
	if patchPropertiesResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on profile patch, got: %d", patchPropertiesResponseRecorder.Code)
	}

	// 5. Convert Anonymous User to Verified User by inserting/linking an email
	claimedEmail := "e2e.converted@example.com"
	_, err = db.Exec(ctx, `
		UPDATE auth.users
		SET email = $1, is_anonymous = false, email_verified_at = clock_timestamp(), last_updated_at = clock_timestamp()
		WHERE id = $2
	`, claimedEmail, anonUserID)
	if err != nil {
		t.Fatalf("failed to update user email: %v", err)
	}

	// 6. Set Initial Password on now-identified user (no previous password needed)
	initialUserPassword := "ConvertedPassword123!#"
	initialSetPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		NewPassword: initialUserPassword,
	})
	initialSetPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(initialSetPasswordPayload))
	initialSetPasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	initialSetPasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(initialSetPasswordResponseRecorder, initialSetPasswordRequest)
	if initialSetPasswordResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content setting initial password, got: %d (%s)", initialSetPasswordResponseRecorder.Code, initialSetPasswordResponseRecorder.Body.String())
	}

	// 7. Rotate Password with valid current password
	rotatedPassword := "BrandNewRotatedPassword123!#"
	rotatePasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: initialUserPassword,
		NewPassword:     rotatedPassword,
	})
	rotatePasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(rotatePasswordPayload))
	rotatePasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	rotatePasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(rotatePasswordResponseRecorder, rotatePasswordRequest)
	if rotatePasswordResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content rotating password, got: %d (%s)", rotatePasswordResponseRecorder.Code, rotatePasswordResponseRecorder.Body.String())
	}

	// 8. Sign In with new rotated password
	signinPayload, _ := json.Marshal(map[string]any{
		"email":    claimedEmail,
		"password": rotatedPassword,
	})
	signinRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signinPayload))
	signinRequest.Header.Set("Content-Type", "application/json")
	signinRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signinResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(signinResponseRecorder, signinRequest)
	if signinResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK signing in with rotated password, got: %d (%s)", signinResponseRecorder.Code, signinResponseRecorder.Body.String())
	}
	var newSessionResponse SessionResponse
	_ = json.NewDecoder(signinResponseRecorder.Body).Decode(&newSessionResponse)

	// 9. Inspect profile again - verify is_anonymous is false, properties preserved
	convertedProfileRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/user", nil)
	convertedProfileRequest.Header.Set("Authorization", "Bearer "+newSessionResponse.AccessToken)
	convertedProfileResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(convertedProfileResponseRecorder, convertedProfileRequest)
	if convertedProfileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from profile, got: %d", convertedProfileResponseRecorder.Code)
	}
	var convertedProfileUserResponse UserResponse
	_ = json.NewDecoder(convertedProfileResponseRecorder.Body).Decode(&convertedProfileUserResponse)
	if convertedProfileUserResponse.IsAnonymous || convertedProfileUserResponse.Properties["display_name"] != "E2E Ghost" {
		t.Fatalf("invalid converted profile state: %+v", convertedProfileUserResponse)
	}

	// 10. Self-Delete Account
	deleteAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/user", nil)
	deleteAccountRequest.Header.Set("Authorization", "Bearer "+newSessionResponse.AccessToken)
	deleteAccountResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(deleteAccountResponseRecorder, deleteAccountRequest)
	if deleteAccountResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on self deletion, got: %d", deleteAccountResponseRecorder.Code)
	}

	// Verify user cannot log in anymore
	postDeleteSigninRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signinPayload))
	postDeleteSigninRequest.Header.Set("Content-Type", "application/json")
	postDeleteSigninRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	postDeleteSigninResponseRecorder := httptest.NewRecorder()
	coreServer.Mux().ServeHTTP(postDeleteSigninResponseRecorder, postDeleteSigninRequest)
	if postDeleteSigninResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for deleted user login, got: %d", postDeleteSigninResponseRecorder.Code)
	}
}
