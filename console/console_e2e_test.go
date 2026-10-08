// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleFullLifecycleE2E(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Console Service and start runtime
	consoleService := NewService(kernel)
	require.NoError(t, consoleService.Start(ctx))
	defer consoleService.Stop()

	require.NotNil(t, consoleService.Kernel())
	require.NotNil(t, consoleService.ConfigManager())
	require.NotNil(t, consoleService.ControlPlaneHandler())

	coreServer := core.NewServer(kernel)
	consoleService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// 2. Create Service Account with full console control plane scopes
	createdServiceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "console-e2e-service-account",
		Scopes: []string{
			core.ScopeConsoleUserRead,
			core.ScopeConsoleUserWrite,
			core.ScopeConsoleConfigRead,
			core.ScopeConsoleConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := createdServiceAccount.SecretKey

	// 3. Control Plane: Inspect and update dynamic runtime configuration
	getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/config", nil)
	getConfigRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	getConfigResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getConfigResponseRecorder, getConfigRequest)
	require.Equal(t, http.StatusOK, getConfigResponseRecorder.Code)

	var activeConfig Config
	require.NoError(t, json.Unmarshal(getConfigResponseRecorder.Body.Bytes(), &activeConfig))
	require.Positive(t, activeConfig.SessionIdleTimeoutSeconds)

	// Mutate configuration
	activeConfig.SessionIdleTimeoutSeconds = 3600
	activeConfig.AllowedCIDRs = []string{"127.0.0.1/32"}
	updateConfigPayload, _ := json.Marshal(activeConfig)
	updateConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/console/config", bytes.NewReader(updateConfigPayload))
	updateConfigRequest.Header.Set("Content-Type", "application/json")
	updateConfigRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	updateConfigResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(updateConfigResponseRecorder, updateConfigRequest)
	require.Equal(t, http.StatusOK, updateConfigResponseRecorder.Code)

	var updatedConfig Config
	require.NoError(t, json.Unmarshal(updateConfigResponseRecorder.Body.Bytes(), &updatedConfig))
	require.Equal(t, 3600, updatedConfig.SessionIdleTimeoutSeconds)

	// 4. Control Plane: Create a new Console Operator User
	userEmail := "platform-admin@layr.sh"
	initialPassword := "PlatformPassword123!#"
	createUserPayload := fmt.Sprintf(`{"email":"%s","password":"%s"}`, userEmail, initialPassword)
	createUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(createUserPayload)))
	createUserRequest.Header.Set("Content-Type", "application/json")
	createUserRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	createUserResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createUserResponseRecorder, createUserRequest)
	require.Equal(t, http.StatusCreated, createUserResponseRecorder.Code)

	var createdUser User
	require.NoError(t, json.Unmarshal(createUserResponseRecorder.Body.Bytes(), &createdUser))
	require.Equal(t, userEmail, createdUser.Email)
	require.NotEmpty(t, createdUser.ID)

	// 5. Control Plane: List and Get Console Users
	listUsersRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users", nil)
	listUsersRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	listUsersResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listUsersResponseRecorder, listUsersRequest)
	require.Equal(t, http.StatusOK, listUsersResponseRecorder.Code)

	var listConsoleUsersResponse ListConsoleUsersResponse
	require.NoError(t, json.Unmarshal(listUsersResponseRecorder.Body.Bytes(), &listConsoleUsersResponse))
	require.GreaterOrEqual(t, len(listConsoleUsersResponse), 1)

	getUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/v1/_/console/users/%s", createdUser.ID), nil)
	getUserRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	getUserResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getUserResponseRecorder, getUserRequest)
	require.Equal(t, http.StatusOK, getUserResponseRecorder.Code)

	var fetchedUser User
	require.NoError(t, json.Unmarshal(getUserResponseRecorder.Body.Bytes(), &fetchedUser))
	require.Equal(t, createdUser.ID, fetchedUser.ID)

	// 6. Control Plane: Update Console User
	updateUserPayload := `{"email":"platform-admin-renamed@layr.sh"}`
	updateUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("/v1/_/console/users/%s", createdUser.ID), bytes.NewReader([]byte(updateUserPayload)))
	updateUserRequest.Header.Set("Content-Type", "application/json")
	updateUserRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	updateUserResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(updateUserResponseRecorder, updateUserRequest)
	require.Equal(t, http.StatusOK, updateUserResponseRecorder.Code)

	// 7. Console Authentication: Sign In as Operator
	signInPayload := fmt.Sprintf(`{"email":"platform-admin-renamed@layr.sh","password":"%s"}`, initialPassword)
	signInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInPayload)))
	signInRequest.Header.Set("Content-Type", "application/json")
	signInResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(signInResponseRecorder, signInRequest)
	require.Equal(t, http.StatusOK, signInResponseRecorder.Code)

	var signInConsoleResponse SignInConsoleResponse
	require.NoError(t, json.Unmarshal(signInResponseRecorder.Body.Bytes(), &signInConsoleResponse))
	require.NotEmpty(t, signInConsoleResponse.AccessToken)

	var sessionCookie *http.Cookie
	for _, cookie := range signInResponseRecorder.Result().Cookies() {
		if cookie.Name == core.ConsoleCookieNameInsecure || cookie.Name == core.ConsoleCookieNameSecure {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// 8. Console Operator: Inspect authenticated session profile
	sessionRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
	sessionRequest.AddCookie(sessionCookie)
	sessionResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(sessionResponseRecorder, sessionRequest)
	require.Equal(t, http.StatusOK, sessionResponseRecorder.Code)

	var getConsoleSessionResponse GetConsoleSessionResponse
	require.NoError(t, json.Unmarshal(sessionResponseRecorder.Body.Bytes(), &getConsoleSessionResponse))
	require.Equal(t, createdUser.ID, getConsoleSessionResponse.User.ID)
	require.NotEmpty(t, getConsoleSessionResponse.Session.ID)

	// 9. Console Operator: Setup, Verify, and Test TOTP MFA Lifecycle
	setupMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/setup", nil)
	setupMFARequest.AddCookie(sessionCookie)
	setupMFAResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(setupMFAResponseRecorder, setupMFARequest)
	require.Equal(t, http.StatusOK, setupMFAResponseRecorder.Code)

	var setupConsoleMFAResponse SetupConsoleMFAResponse
	require.NoError(t, json.Unmarshal(setupMFAResponseRecorder.Body.Bytes(), &setupConsoleMFAResponse))
	require.NotEmpty(t, setupConsoleMFAResponse.Secret)
	require.NotEmpty(t, setupConsoleMFAResponse.URI)

	// Generate valid TOTP code
	validMFACode, mfaCodeErr := consoleService.totpManager.GenerateCode(setupConsoleMFAResponse.Secret, time.Now().UTC())
	require.NoError(t, mfaCodeErr)

	// Verify MFA
	verifyMFAPayload := fmt.Sprintf(`{"code":"%s"}`, validMFACode)
	verifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(verifyMFAPayload)))
	verifyMFARequest.Header.Set("Content-Type", "application/json")
	verifyMFARequest.AddCookie(sessionCookie)
	verifyMFAResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(verifyMFAResponseRecorder, verifyMFARequest)
	require.Equal(t, http.StatusOK, verifyMFAResponseRecorder.Code)

	// Sign In now requires MFA code -> missing MFA returns 401
	missingMFASignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInPayload)))
	missingMFASignInRequest.Header.Set("Content-Type", "application/json")
	missingMFAResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(missingMFAResponseRecorder, missingMFASignInRequest)
	require.Equal(t, http.StatusUnauthorized, missingMFAResponseRecorder.Code)

	// Sign In with valid MFA code -> succeeds
	validMFASignInPayload := fmt.Sprintf(`{"email":"platform-admin-renamed@layr.sh","password":"%s","mfa_code":"%s"}`, initialPassword, validMFACode)
	validMFASignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(validMFASignInPayload)))
	validMFASignInRequest.Header.Set("Content-Type", "application/json")
	validMFAResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(validMFAResponseRecorder, validMFASignInRequest)
	require.Equal(t, http.StatusOK, validMFAResponseRecorder.Code)

	// Disable MFA
	disableMFARequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/mfa", nil)
	disableMFARequest.AddCookie(sessionCookie)
	disableMFAResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(disableMFAResponseRecorder, disableMFARequest)
	require.Equal(t, http.StatusNoContent, disableMFAResponseRecorder.Code)

	// 10. Control Plane: List and Revoke User Sessions
	listSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/v1/_/console/users/%s/sessions", createdUser.ID), nil)
	listSessionsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	listSessionsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listSessionsResponseRecorder, listSessionsRequest)
	require.Equal(t, http.StatusOK, listSessionsResponseRecorder.Code)

	var listConsoleUserSessionsResponse ListConsoleUserSessionsResponse
	require.NoError(t, json.Unmarshal(listSessionsResponseRecorder.Body.Bytes(), &listConsoleUserSessionsResponse))
	require.NotEmpty(t, listConsoleUserSessionsResponse)

	// Delete specific session
	targetSessionID := listConsoleUserSessionsResponse[0].ID
	delSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("/v1/_/console/users/%s/sessions/%s", createdUser.ID, targetSessionID), nil)
	delSessionRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	delSessionResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(delSessionResponseRecorder, delSessionRequest)
	require.Equal(t, http.StatusNoContent, delSessionResponseRecorder.Code)

	// 11. Control Plane: Administratively Reset Password and Revoke All Sessions
	newPassword := "RotatedOperatorKey789!#"
	resetPasswordPayload := fmt.Sprintf(`{"new_password":"%s"}`, newPassword)
	resetPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("/v1/_/console/users/%s/password", createdUser.ID), bytes.NewReader([]byte(resetPasswordPayload)))
	resetPasswordRequest.Header.Set("Content-Type", "application/json")
	resetPasswordRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	resetPasswordResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(resetPasswordResponseRecorder, resetPasswordRequest)
	require.Equal(t, http.StatusOK, resetPasswordResponseRecorder.Code)

	// Authenticate with rotated password
	rotatedSignInPayload := fmt.Sprintf(`{"email":"platform-admin-renamed@layr.sh","password":"%s"}`, newPassword)
	rotatedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(rotatedSignInPayload)))
	rotatedSignInRequest.Header.Set("Content-Type", "application/json")
	rotatedSignInResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(rotatedSignInResponseRecorder, rotatedSignInRequest)
	require.Equal(t, http.StatusOK, rotatedSignInResponseRecorder.Code)

	var newSessionCookie *http.Cookie
	for _, cookie := range rotatedSignInResponseRecorder.Result().Cookies() {
		if cookie.Name == core.ConsoleCookieNameInsecure || cookie.Name == core.ConsoleCookieNameSecure {
			newSessionCookie = cookie
			break
		}
	}
	require.NotNil(t, newSessionCookie)

	// Revoke all sessions
	revokeAllSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("/v1/_/console/users/%s/sessions/revoke", createdUser.ID), nil)
	revokeAllSessionsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	revokeAllSessionsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(revokeAllSessionsResponseRecorder, revokeAllSessionsRequest)
	require.Equal(t, http.StatusNoContent, revokeAllSessionsResponseRecorder.Code)

	// 12. Operator Sign Out
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-out", nil)
	signOutRequest.AddCookie(newSessionCookie)
	signOutResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(signOutResponseRecorder, signOutRequest)
	require.Equal(t, http.StatusNoContent, signOutResponseRecorder.Code)

	// 13. Control Plane: Delete User
	deleteUserRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("/v1/_/console/users/%s", createdUser.ID), nil)
	deleteUserRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	deleteUserResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteUserResponseRecorder, deleteUserRequest)
	require.Equal(t, http.StatusNoContent, deleteUserResponseRecorder.Code)
}
