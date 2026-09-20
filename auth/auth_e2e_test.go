package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
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
	coreServer.Handler().ServeHTTP(signupResponseRecorder, signupRequest)

	if signupResponseRecorder.Code != http.StatusCreated && signupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected sign up 200/201, got %d (body: %s)", signupResponseRecorder.Code, signupResponseRecorder.Body.String())
	}

	var signupAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(signupResponseRecorder.Body).Decode(&signupAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode sign up response: %v", err)
	}
	if signupAuthTokenResponse.User.ID == "" || signupAuthTokenResponse.AccessToken == "" {
		t.Fatalf("invalid sign up payload response: %+v", signupAuthTokenResponse)
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
	coreServer.Handler().ServeHTTP(signinResponseRecorder, signinRequest)

	if signinResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected login 200 OK, got %d (body: %s)", signinResponseRecorder.Code, signinResponseRecorder.Body.String())
	}

	var signinAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(signinResponseRecorder.Body).Decode(&signinAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// 3. Authenticated Profile Access via User Export
	exportRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users/"+signupAuthTokenResponse.User.ID+"/export", nil)
	exportRequest.Header.Set("Authorization", "Bearer "+signinAuthTokenResponse.AccessToken)
	exportResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(exportResponseRecorder, exportRequest)

	if exportResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected user export 200 OK, got %d (body: %s)", exportResponseRecorder.Code, exportResponseRecorder.Body.String())
	}

	// 4. Token Refresh
	refreshPayload, _ := json.Marshal(map[string]any{
		"refresh_token": signinAuthTokenResponse.RefreshToken,
	})
	refreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(refreshPayload))
	refreshRequest.Header.Set("Content-Type", "application/json")
	refreshRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	refreshResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(refreshResponseRecorder, refreshRequest)

	if refreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected refresh 200 OK, got %d (body: %s)", refreshResponseRecorder.Code, refreshResponseRecorder.Body.String())
	}

	var refreshedAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(refreshResponseRecorder.Body).Decode(&refreshedAuthTokenResponse)

	// 5. Sign Out / Session Revocation
	signoutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-out", nil)
	signoutRequest.Header.Set("Authorization", "Bearer "+refreshedAuthTokenResponse.AccessToken)
	signoutRequest.AddCookie(&http.Cookie{Name: core.SessionCookieNameInsecure, Value: refreshedAuthTokenResponse.RefreshToken})
	signoutResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(signoutResponseRecorder, signoutRequest)

	if signoutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout 200 OK, got %d (body: %s)", signoutResponseRecorder.Code, signoutResponseRecorder.Body.String())
	}

	// 6. Old Refresh Token Must Now Fail
	revokedRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(refreshPayload))
	revokedRefreshRequest.Header.Set("Content-Type", "application/json")
	revokedRefreshRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	revokedRefreshResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(revokedRefreshResponseRecorder, revokedRefreshRequest)

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
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
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
	coreServer.Handler().ServeHTTP(signupResponseRecorder, signupRequest)
	if signupResponseRecorder.Code != http.StatusOK && signupResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 on sign up: %d", signupResponseRecorder.Code)
	}
	var firstAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(signupResponseRecorder.Body).Decode(&firstAuthTokenResponse)

	// 2. Open secondary session (simulating second device)
	signInPayload, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": userPassword,
	})
	signInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signInPayload))
	signInRequest.Header.Set("Content-Type", "application/json")
	signInRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	signInRequest.Header.Set("User-Agent", "Secondary-Device-Tablet")
	signInResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(signInResponseRecorder, signInRequest)
	if signInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on second device sign-in, got: %d", signInResponseRecorder.Code)
	}
	var secondAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(signInResponseRecorder.Body).Decode(&secondAuthTokenResponse)

	// 3. List active sessions via E2E server mux
	listSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/sessions", nil)
	listSessionsRequest.Header.Set("Authorization", "Bearer "+firstAuthTokenResponse.AccessToken)
	listSessionsRequest.AddCookie(&http.Cookie{Name: core.SessionCookieNameInsecure, Value: firstAuthTokenResponse.RefreshToken})
	listSessionsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listSessionsResponseRecorder, listSessionsRequest)
	if listSessionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list sessions, got: %d", listSessionsResponseRecorder.Code)
	}
	var listSessionsResponse ListSessionsResponse
	_ = json.NewDecoder(listSessionsResponseRecorder.Body).Decode(&listSessionsResponse)
	if listSessionsResponse.Count != 2 {
		t.Fatalf("expected 2 sessions, got: %d", listSessionsResponse.Count)
	}

	var secondarySessionID string
	for _, userSession := range listSessionsResponse.Sessions {
		if userSession.UserAgent != nil && *userSession.UserAgent == "Secondary-Device-Tablet" {
			secondarySessionID = userSession.ID
		}
	}
	if secondarySessionID == "" {
		t.Fatalf("expected to find secondary device session ID")
	}

	// 4. Revoke secondary session via E2E server mux
	deleteSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/sessions/"+secondarySessionID, nil)
	deleteSessionRequest.Header.Set("Authorization", "Bearer "+firstAuthTokenResponse.AccessToken)
	deleteSessionResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteSessionResponseRecorder, deleteSessionRequest)
	if deleteSessionResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete session: %d", deleteSessionResponseRecorder.Code)
	}

	// 5. Revoke other sessions via E2E server mux (now only 1 session left)
	revokeOthersRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sessions/revoke-others", nil)
	revokeOthersRequest.Header.Set("Authorization", "Bearer "+firstAuthTokenResponse.AccessToken)
	revokeOthersRequest.AddCookie(&http.Cookie{Name: core.SessionCookieNameInsecure, Value: firstAuthTokenResponse.RefreshToken})
	revokeOthersResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(revokeOthersResponseRecorder, revokeOthersRequest)
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
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
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

	currentAccessToken, err := service.baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
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
	coreServer.Handler().ServeHTTP(initialProfileResponseRecorder, initialProfileRequest)
	if initialProfileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from profile, got: %d (%s)", initialProfileResponseRecorder.Code, initialProfileResponseRecorder.Body.String())
	}
	var initialGetUserResponse GetUserResponse
	_ = json.NewDecoder(initialProfileResponseRecorder.Body).Decode(&initialGetUserResponse)
	if !initialGetUserResponse.IsAnonymous {
		t.Fatalf("expected is_anonymous to be true")
	}

	// 3. Attempting to set password on anonymous user MUST FAIL (400 Bad Request)
	forbiddenPasswordPayload, _ := json.Marshal(UpdateUserPasswordInput{
		NewPassword: "AttemptedPassword123!",
	})
	forbiddenPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(forbiddenPasswordPayload))
	forbiddenPasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	forbiddenPasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(forbiddenPasswordResponseRecorder, forbiddenPasswordRequest)
	if forbiddenPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on anonymous password change, got: %d (%s)", forbiddenPasswordResponseRecorder.Code, forbiddenPasswordResponseRecorder.Body.String())
	}

	// 4. Update Profile Properties
	patchPropertiesPayload, _ := json.Marshal(UpdateUserPropertiesInput{
		Properties: map[string]any{
			"display_name": "E2E Ghost",
			"theme":        "dark",
		},
	})
	patchPropertiesRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/properties", bytes.NewReader(patchPropertiesPayload))
	patchPropertiesRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	patchPropertiesResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(patchPropertiesResponseRecorder, patchPropertiesRequest)
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
	initialSetPasswordPayload, _ := json.Marshal(UpdateUserPasswordInput{
		NewPassword: initialUserPassword,
	})
	initialSetPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(initialSetPasswordPayload))
	initialSetPasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	initialSetPasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(initialSetPasswordResponseRecorder, initialSetPasswordRequest)
	if initialSetPasswordResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content setting initial password, got: %d (%s)", initialSetPasswordResponseRecorder.Code, initialSetPasswordResponseRecorder.Body.String())
	}

	// 7. Rotate Password with valid current password
	rotatedPassword := "BrandNewRotatedPassword123!#"
	rotatePasswordPayload, _ := json.Marshal(UpdateUserPasswordInput{
		CurrentPassword: initialUserPassword,
		NewPassword:     rotatedPassword,
	})
	rotatePasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(rotatePasswordPayload))
	rotatePasswordRequest.Header.Set("Authorization", "Bearer "+currentAccessToken)
	rotatePasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(rotatePasswordResponseRecorder, rotatePasswordRequest)
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
	coreServer.Handler().ServeHTTP(signinResponseRecorder, signinRequest)
	if signinResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK signing in with rotated password, got: %d (%s)", signinResponseRecorder.Code, signinResponseRecorder.Body.String())
	}
	var newAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(signinResponseRecorder.Body).Decode(&newAuthTokenResponse)

	// 9. Inspect profile again - verify is_anonymous is false, properties preserved
	convertedProfileRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/user", nil)
	convertedProfileRequest.Header.Set("Authorization", "Bearer "+newAuthTokenResponse.AccessToken)
	convertedProfileResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(convertedProfileResponseRecorder, convertedProfileRequest)
	if convertedProfileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from profile, got: %d", convertedProfileResponseRecorder.Code)
	}
	var convertedProfileGetUserResponse GetUserResponse
	_ = json.NewDecoder(convertedProfileResponseRecorder.Body).Decode(&convertedProfileGetUserResponse)
	if convertedProfileGetUserResponse.IsAnonymous || convertedProfileGetUserResponse.Properties["display_name"] != "E2E Ghost" {
		t.Fatalf("invalid converted profile state: %+v", convertedProfileGetUserResponse)
	}

	// 10. Self-Delete Account
	deleteAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/user", nil)
	deleteAccountRequest.Header.Set("Authorization", "Bearer "+newAuthTokenResponse.AccessToken)
	deleteAccountResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteAccountResponseRecorder, deleteAccountRequest)
	if deleteAccountResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on self deletion, got: %d", deleteAccountResponseRecorder.Code)
	}

	// Verify user cannot log in anymore
	postDeleteSigninRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signinPayload))
	postDeleteSigninRequest.Header.Set("Content-Type", "application/json")
	postDeleteSigninRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	postDeleteSigninResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(postDeleteSigninResponseRecorder, postDeleteSigninRequest)
	if postDeleteSigninResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for deleted user login, got: %d", postDeleteSigninResponseRecorder.Code)
	}
}
