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

func TestConsoleControlPlaneHandlerAuthIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// Create test console user in DB
	hasher := service.hasher
	passwordHash, err := hasher.Hash("ConsolePass123!")
	require.NoError(t, err)

	insertUserSQL := `
		INSERT INTO console.users (email, password_hash, is_enabled)
		VALUES ('operator@layr.sh', $1, true)
		RETURNING id
	`
	var userID string
	require.NoError(t, kernel.DB().QueryRow(ctx, insertUserSQL, passwordHash).Scan(&userID))

	var activeSessionCookie *http.Cookie
	var mfaSecret string

	t.Run("full authentication and MFA lifecycle", func(t *testing.T) {
		// 1. Sign In
		signInPayload := `{"email":"operator@layr.sh","password":"ConsolePass123!"}`
		signInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInPayload)))
		signInRequest.Header.Set("Content-Type", "application/json")
		signInResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(signInResponseRecorder, signInRequest)
		require.Equal(t, http.StatusOK, signInResponseRecorder.Code)

		var signInConsoleResponse SignInConsoleResponse
		require.NoError(t, json.Unmarshal(signInResponseRecorder.Body.Bytes(), &signInConsoleResponse))
		require.Equal(t, "operator@layr.sh", signInConsoleResponse.User.Email)
		require.NotEmpty(t, signInConsoleResponse.AccessToken)

		cookies := signInResponseRecorder.Result().Cookies()
		for _, c := range cookies {
			if c.Name == core.ConsoleCookieNameInsecure || c.Name == core.ConsoleCookieNameSecure {
				activeSessionCookie = c
				break
			}
		}
		require.NotNil(t, activeSessionCookie)

		// Test HTTPS sign-in to cover ConsoleCookieNameSecure branch
		secureSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInPayload)))
		secureSignInRequest.Header.Set("Content-Type", "application/json")
		secureSignInRequest.Header.Set("X-Forwarded-Proto", "https")
		secureSignInResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(secureSignInResponseRecorder, secureSignInRequest)
		require.Equal(t, http.StatusOK, secureSignInResponseRecorder.Code)
		var hasSecureCookie bool
		for _, c := range secureSignInResponseRecorder.Result().Cookies() {
			if c.Name == core.ConsoleCookieNameSecure {
				hasSecureCookie = true
			}
		}
		require.True(t, hasSecureCookie)

		// 2. Get Session with Cookie
		getSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		getSessionRequest.AddCookie(activeSessionCookie)
		getSessionResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getSessionResponseRecorder, getSessionRequest)
		require.Equal(t, http.StatusOK, getSessionResponseRecorder.Code)

		var getConsoleSessionResponse GetConsoleSessionResponse
		require.NoError(t, json.Unmarshal(getSessionResponseRecorder.Body.Bytes(), &getConsoleSessionResponse))
		require.Equal(t, userID, getConsoleSessionResponse.User.ID)
		require.NotEmpty(t, getConsoleSessionResponse.Scopes)

		// 3. Initiate MFA Setup
		setupMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa", nil)
		setupMFARequest.AddCookie(activeSessionCookie)
		setupMFAResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(setupMFAResponseRecorder, setupMFARequest)
		require.Equal(t, http.StatusOK, setupMFAResponseRecorder.Code)

		var setupConsoleMFAResponse SetupConsoleMFAResponse
		require.NoError(t, json.Unmarshal(setupMFAResponseRecorder.Body.Bytes(), &setupConsoleMFAResponse))
		require.NotEmpty(t, setupConsoleMFAResponse.Secret)
		require.NotEmpty(t, setupConsoleMFAResponse.URI)
		mfaSecret = setupConsoleMFAResponse.Secret

		// 4. Verify MFA with Code
		totpCode, codeErr := service.totpManager.GenerateCode(mfaSecret, time.Now().UTC())
		require.NoError(t, codeErr)

		verifyPayload := `{"code":"` + totpCode + `"}`
		verifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(verifyPayload)))
		verifyMFARequest.Header.Set("Content-Type", "application/json")
		verifyMFARequest.AddCookie(activeSessionCookie)
		verifyMFAResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(verifyMFAResponseRecorder, verifyMFARequest)
		require.Equal(t, http.StatusOK, verifyMFAResponseRecorder.Code)

		// 5. Sign In again without MFA code -> returns 200 with MFARequired: true
		signInNoMFAResponseRecorder := httptest.NewRecorder()
		signInNoMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInPayload)))
		signInNoMFARequest.Header.Set("Content-Type", "application/json")
		coreServer.Handler().ServeHTTP(signInNoMFAResponseRecorder, signInNoMFARequest)
		require.Equal(t, http.StatusOK, signInNoMFAResponseRecorder.Code)

		var mfaPromptSignInConsoleResponse SignInConsoleResponse
		require.NoError(t, json.Unmarshal(signInNoMFAResponseRecorder.Body.Bytes(), &mfaPromptSignInConsoleResponse))
		require.True(t, mfaPromptSignInConsoleResponse.MFARequired)
		require.NotEmpty(t, mfaPromptSignInConsoleResponse.MFATicket)

		// 5.1 Complete 2FA via /v1/_/console/mfa/challenge -> 200
		challengeCode, challengeCodeErr := service.totpManager.GenerateCode(mfaSecret, time.Now().UTC())
		require.NoError(t, challengeCodeErr)
		challengePayload := fmt.Sprintf(`{"mfa_ticket":"%s","code":"%s"}`, mfaPromptSignInConsoleResponse.MFATicket, challengeCode)
		challengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(challengePayload)))
		challengeRequest.Header.Set("Content-Type", "application/json")
		challengeResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(challengeResponseRecorder, challengeRequest)
		require.Equal(t, http.StatusOK, challengeResponseRecorder.Code)

		// 6. Sign In again with invalid MFA code -> 401
		signInBadMFAPayload := `{"email":"operator@layr.sh","password":"ConsolePass123!","mfa_code":"000000"}`
		signInBadMFAResponseRecorder := httptest.NewRecorder()
		signInBadMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInBadMFAPayload)))
		signInBadMFARequest.Header.Set("Content-Type", "application/json")
		coreServer.Handler().ServeHTTP(signInBadMFAResponseRecorder, signInBadMFARequest)
		require.Equal(t, http.StatusUnauthorized, signInBadMFAResponseRecorder.Code)

		// 7. Sign In again with valid MFA code -> 200
		newTOTPCode, newCodeErr := service.totpManager.GenerateCode(mfaSecret, time.Now().UTC())
		require.NoError(t, newCodeErr)

		signInValidMFAPayload := `{"email":"operator@layr.sh","password":"ConsolePass123!","mfa_code":"` + newTOTPCode + `"}`
		signInValidMFAResponseRecorder := httptest.NewRecorder()
		signInValidMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(signInValidMFAPayload)))
		signInValidMFARequest.Header.Set("Content-Type", "application/json")
		coreServer.Handler().ServeHTTP(signInValidMFAResponseRecorder, signInValidMFARequest)
		require.Equal(t, http.StatusOK, signInValidMFAResponseRecorder.Code)

		// 8. Disable MFA
		disableMFARequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/mfa", nil)
		disableMFARequest.AddCookie(activeSessionCookie)
		disableMFAResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(disableMFAResponseRecorder, disableMFARequest)
		require.Equal(t, http.StatusNoContent, disableMFAResponseRecorder.Code)

		// 9. Sign Out
		signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-out", nil)
		signOutRequest.AddCookie(activeSessionCookie)
		signOutResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(signOutResponseRecorder, signOutRequest)
		require.Equal(t, http.StatusNoContent, signOutResponseRecorder.Code)

		// 10. Verify old session is revoked
		getSessionAfterRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		getSessionAfterRequest.AddCookie(activeSessionCookie)
		getSessionAfterResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getSessionAfterResponseRecorder, getSessionAfterRequest)
		require.Equal(t, http.StatusUnauthorized, getSessionAfterResponseRecorder.Code)
	})
}
