package console

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleControlPlaneHandlerAuthUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	ctx := context.Background()

	t.Run("authenticateSession error branches", func(t *testing.T) {
		// Missing token
		missingTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		user, session, err := controlPlaneHandler.authenticateSession(missingTokenRequest)
		require.Error(t, err)
		require.Nil(t, user)
		require.Nil(t, session)
		require.Equal(t, "authentication required", err.Error())

		// Nil request
		require.Empty(t, controlPlaneHandler.extractConsoleToken(nil))

		// Secure cookie extraction
		secureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		secureRequest.AddCookie(&http.Cookie{Name: core.ConsoleCookieNameSecure, Value: "secure-token"})
		require.Equal(t, "secure-token", controlPlaneHandler.extractConsoleToken(secureRequest))

		// Insecure cookie extraction
		insecureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		insecureRequest.AddCookie(&http.Cookie{Name: core.ConsoleCookieNameInsecure, Value: "insecure-token"})
		require.Equal(t, "insecure-token", controlPlaneHandler.extractConsoleToken(insecureRequest))

		// Broken DB query
		withTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		withTokenRequest.Header.Set("Authorization", "Bearer invalid-token")
		user, session, err = controlPlaneHandler.authenticateSession(withTokenRequest)
		require.Error(t, err)
		require.Nil(t, user)
		require.Nil(t, session)
		require.Equal(t, "invalid console token", err.Error())
	})

	t.Run("handleSignInConsole validation errors", func(t *testing.T) {
		// Bad JSON
		badJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(`{invalid`)))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Empty fields
		emptyFieldsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(`{"email":"","password":""}`)))
		emptyFieldsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(emptyFieldsResponseRecorder, emptyFieldsRequest)
		require.Equal(t, http.StatusBadRequest, emptyFieldsResponseRecorder.Code)

		// Broken DB query
		brokenDBRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(`{"email":"admin@example.com","password":"secret"}`)))
		brokenDBResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(brokenDBResponseRecorder, brokenDBRequest)
		require.Equal(t, http.StatusUnauthorized, brokenDBResponseRecorder.Code)
	})

	t.Run("handleSignOutConsole without token", func(t *testing.T) {
		noTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-out", nil)
		noTokenResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignOutConsole(noTokenResponseRecorder, noTokenRequest)
		require.Equal(t, http.StatusNoContent, noTokenResponseRecorder.Code)
	})

	t.Run("unauthenticated handlers return 401", func(t *testing.T) {
		// handleGetConsoleSession
		sessionRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		sessionResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConsoleSession(sessionResponseRecorder, sessionRequest)
		require.Equal(t, http.StatusUnauthorized, sessionResponseRecorder.Code)

		// handleSetupConsoleMFA
		setupMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa", nil)
		setupMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSetupConsoleMFA(setupMFAResponseRecorder, setupMFARequest)
		require.Equal(t, http.StatusUnauthorized, setupMFAResponseRecorder.Code)

		// handleVerifyConsoleMFA
		verifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", nil)
		verifyMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleVerifyConsoleMFA(verifyMFAResponseRecorder, verifyMFARequest)
		require.Equal(t, http.StatusUnauthorized, verifyMFAResponseRecorder.Code)

		// handleDisableConsoleMFA
		disableMFARequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/mfa", nil)
		disableMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDisableConsoleMFA(disableMFAResponseRecorder, disableMFARequest)
		require.Equal(t, http.StatusUnauthorized, disableMFAResponseRecorder.Code)
	})
}

func TestConsoleControlPlaneHandlerAuthDatabaseUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Insert test disabled user
	hasher := service.hasher
	pwdHash, err := hasher.Hash("ValidPass123!")
	require.NoError(t, err)

	var disabledUserID string
	insertDisabledUserSQL := `
		INSERT INTO console.users (email, password_hash, is_enabled)
		VALUES ('disabled@example.com', $1, false)
		RETURNING id
	`
	require.NoError(t, kernel.DB().QueryRow(ctx, insertDisabledUserSQL, pwdHash).Scan(&disabledUserID))

	t.Run("disabled account sign in returns 403", func(t *testing.T) {
		payload := `{"email":"disabled@example.com","password":"ValidPass123!"}`
		disabledSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(payload)))
		disabledSignInResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(disabledSignInResponseRecorder, disabledSignInRequest)
		require.Equal(t, http.StatusForbidden, disabledSignInResponseRecorder.Code)
	})

	t.Run("wrong password sign in returns 401", func(t *testing.T) {
		payload := `{"email":"disabled@example.com","password":"WrongPassword!"}`
		wrongPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(payload)))
		wrongPasswordResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(wrongPasswordResponseRecorder, wrongPasswordRequest)
		require.Equal(t, http.StatusUnauthorized, wrongPasswordResponseRecorder.Code)
	})

	t.Run("expired session returns error", func(t *testing.T) {
		rawToken := kernel.JWTSigner().GenerateRefreshToken()
		tokenHash := kernel.JWTSigner().HashRefreshToken(rawToken)

		insertExpiredSessionSQL := `
			INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, '127.0.0.1'::inet, 'Go-Test', $3)
		`
		pastTime := time.Now().UTC().Add(-time.Hour)
		_, insertErr := kernel.DB().Exec(ctx, insertExpiredSessionSQL, disabledUserID, tokenHash, pastTime)
		require.NoError(t, insertErr)

		expiredSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		expiredSessionRequest.Header.Set("Authorization", "Bearer "+rawToken)
		user, session, authErr := controlPlaneHandler.authenticateSession(expiredSessionRequest)
		require.Error(t, authErr)
		require.Nil(t, user)
		require.Nil(t, session)
		require.Equal(t, "session expired", authErr.Error())
	})

	t.Run("active session on disabled user returns account disabled", func(t *testing.T) {
		rawToken := kernel.JWTSigner().GenerateRefreshToken()
		tokenHash := kernel.JWTSigner().HashRefreshToken(rawToken)

		insertSessionSQL := `
			INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, '127.0.0.1'::inet, 'Go-Test', $3)
		`
		futureTime := time.Now().UTC().Add(time.Hour)
		_, insertErr := kernel.DB().Exec(ctx, insertSessionSQL, disabledUserID, tokenHash, futureTime)
		require.NoError(t, insertErr)

		disabledUserSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/session", nil)
		disabledUserSessionRequest.Header.Set("Authorization", "Bearer "+rawToken)
		user, session, authErr := controlPlaneHandler.authenticateSession(disabledUserSessionRequest)
		require.Error(t, authErr)
		require.Nil(t, user)
		require.Nil(t, session)
		require.Equal(t, "account disabled", authErr.Error())
	})

	t.Run("mfa validation error branches", func(t *testing.T) {
		var enabledUserID string
		insertEnabledUserSQL := `
			INSERT INTO console.users (email, password_hash, is_enabled)
			VALUES ('mfa-unit@example.com', $1, true)
			RETURNING id
		`
		require.NoError(t, kernel.DB().QueryRow(ctx, insertEnabledUserSQL, pwdHash).Scan(&enabledUserID))

		activeToken := kernel.JWTSigner().GenerateRefreshToken()
		activeTokenHash := kernel.JWTSigner().HashRefreshToken(activeToken)
		insertSessionSQL := `
			INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, '127.0.0.1'::inet, 'Go-Test', $3)
		`
		_, sessionErr := kernel.DB().Exec(ctx, insertSessionSQL, enabledUserID, activeTokenHash, time.Now().UTC().Add(time.Hour))
		require.NoError(t, sessionErr)

		authHeader := "Bearer " + activeToken

		// verify MFA bad JSON -> 400
		badJSONVerifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(`{invalid`)))
		badJSONVerifyMFARequest.Header.Set("Authorization", authHeader)
		badJSONVerifyMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleVerifyConsoleMFA(badJSONVerifyMFAResponseRecorder, badJSONVerifyMFARequest)
		require.Equal(t, http.StatusBadRequest, badJSONVerifyMFAResponseRecorder.Code)

		// verify MFA when setup not initiated -> 400
		noSetupVerifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(`{"code":"123456"}`)))
		noSetupVerifyMFARequest.Header.Set("Authorization", authHeader)
		noSetupVerifyMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleVerifyConsoleMFA(noSetupVerifyMFAResponseRecorder, noSetupVerifyMFARequest)
		require.Equal(t, http.StatusBadRequest, noSetupVerifyMFAResponseRecorder.Code)

		// set unverified secret in KV store
		secret, _ := service.totpManager.GenerateSecret()
		_ = kernel.KVStore().Set(ctx, "console:mfa_setup:"+enabledUserID, secret, time.Hour)

		// verify MFA with invalid code -> 400
		badCodeVerifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(`{"code":"000000"}`)))
		badCodeVerifyMFARequest.Header.Set("Authorization", authHeader)
		badCodeVerifyMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleVerifyConsoleMFA(badCodeVerifyMFAResponseRecorder, badCodeVerifyMFARequest)
		require.Equal(t, http.StatusBadRequest, badCodeVerifyMFAResponseRecorder.Code)

		// verify MFA with valid code -> 200
		validCode, _ := service.totpManager.GenerateCode(secret, time.Now().UTC())
		validCodeVerifyMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/verify", bytes.NewReader([]byte(`{"code":"`+validCode+`"}`)))
		validCodeVerifyMFARequest.Header.Set("Authorization", authHeader)
		validCodeVerifyMFAResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleVerifyConsoleMFA(validCodeVerifyMFAResponseRecorder, validCodeVerifyMFARequest)
		require.Equal(t, http.StatusOK, validCodeVerifyMFAResponseRecorder.Code)

		// sign in MFA enabled missing code -> 200 OK with MFARequired: true and MFATicket
		missingMFASignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(`{"email":"mfa-unit@example.com","password":"ValidPass123!"}`)))
		missingMFASignInResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(missingMFASignInResponseRecorder, missingMFASignInRequest)
		require.Equal(t, http.StatusOK, missingMFASignInResponseRecorder.Code)

		var mfaPromptSignInConsoleResponse SignInConsoleResponse
		require.NoError(t, json.Unmarshal(missingMFASignInResponseRecorder.Body.Bytes(), &mfaPromptSignInConsoleResponse))
		require.True(t, mfaPromptSignInConsoleResponse.MFARequired)
		require.NotEmpty(t, mfaPromptSignInConsoleResponse.MFATicket)

		// challenge with bad JSON -> 400
		badJSONChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{invalid`)))
		badJSONChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(badJSONChallengeResponseRecorder, badJSONChallengeRequest)
		require.Equal(t, http.StatusBadRequest, badJSONChallengeResponseRecorder.Code)

		// challenge with missing fields -> 400
		missingFieldsChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"","code":""}`)))
		missingFieldsChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(missingFieldsChallengeResponseRecorder, missingFieldsChallengeRequest)
		require.Equal(t, http.StatusBadRequest, missingFieldsChallengeResponseRecorder.Code)

		// challenge with invalid ticket -> 401
		invalidTicketChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"bogus","code":"123456"}`)))
		invalidTicketChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(invalidTicketChallengeResponseRecorder, invalidTicketChallengeRequest)
		require.Equal(t, http.StatusUnauthorized, invalidTicketChallengeResponseRecorder.Code)

		// challenge with non-existent user -> 401
		ghostTicket := "console_mfa_tk_ghost"
		_ = kernel.KVStore().Set(ctx, "console:mfa_ticket:"+ghostTicket, "01918a24-9999-7000-8000-000000000009", time.Minute)
		ghostChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"`+ghostTicket+`","code":"123456"}`)))
		ghostChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(ghostChallengeResponseRecorder, ghostChallengeRequest)
		require.Equal(t, http.StatusUnauthorized, ghostChallengeResponseRecorder.Code)

		// challenge with disabled user -> 403
		disabledTicket := "console_mfa_tk_disabled"
		_ = kernel.KVStore().Set(ctx, "console:mfa_ticket:"+disabledTicket, disabledUserID, time.Minute)
		disabledChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"`+disabledTicket+`","code":"123456"}`)))
		disabledChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(disabledChallengeResponseRecorder, disabledChallengeRequest)
		require.Equal(t, http.StatusForbidden, disabledChallengeResponseRecorder.Code)

		// challenge with user having no MFA enabled -> 400
		var noMFAUserID string
		insertNoMFAUserSQL := `
			INSERT INTO console.users (email, password_hash, is_enabled)
			VALUES ('nomfa-unit@example.com', $1, true)
			RETURNING id
		`
		require.NoError(t, kernel.DB().QueryRow(ctx, insertNoMFAUserSQL, pwdHash).Scan(&noMFAUserID))
		noMFATicket := "console_mfa_tk_nomfa"
		_ = kernel.KVStore().Set(ctx, "console:mfa_ticket:"+noMFATicket, noMFAUserID, time.Minute)
		noMFAChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"`+noMFATicket+`","code":"123456"}`)))
		noMFAChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(noMFAChallengeResponseRecorder, noMFAChallengeRequest)
		require.Equal(t, http.StatusBadRequest, noMFAChallengeResponseRecorder.Code)

		// challenge with valid ticket and bad code -> 401
		_ = kernel.KVStore().Set(ctx, "console:mfa_ticket:"+mfaPromptSignInConsoleResponse.MFATicket, enabledUserID, time.Minute)
		badCodeChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"`+mfaPromptSignInConsoleResponse.MFATicket+`","code":"000000"}`)))
		badCodeChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(badCodeChallengeResponseRecorder, badCodeChallengeRequest)
		require.Equal(t, http.StatusUnauthorized, badCodeChallengeResponseRecorder.Code)

		// challenge with valid ticket and valid code -> 200
		validChallengeCode, _ := service.totpManager.GenerateCode(secret, time.Now().UTC())
		_ = kernel.KVStore().Set(ctx, "console:mfa_ticket:"+mfaPromptSignInConsoleResponse.MFATicket, enabledUserID, time.Minute)
		validChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/mfa/challenge", bytes.NewReader([]byte(`{"mfa_ticket":"`+mfaPromptSignInConsoleResponse.MFATicket+`","code":"`+validChallengeCode+`"}`)))
		validChallengeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleChallengeConsoleMFA(validChallengeResponseRecorder, validChallengeRequest)
		require.Equal(t, http.StatusOK, validChallengeResponseRecorder.Code)

		// sign in MFA enabled bad code -> 401
		badCode := "000000"
		badMFASignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/sign-in", bytes.NewReader([]byte(`{"email":"mfa-unit@example.com","password":"ValidPass123!","mfa_code":"`+badCode+`"}`)))
		badMFASignInResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignInConsole(badMFASignInResponseRecorder, badMFASignInRequest)
		require.Equal(t, http.StatusUnauthorized, badMFASignInResponseRecorder.Code)
	})
}
