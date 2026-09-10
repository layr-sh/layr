package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthHandlerInitializationUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)
	if handler.kvStore != testKVStore {
		t.Fatal("expected KVStore to be set")
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{}
	}, cryptoKeyManager)
	handler.SetEmailDispatcher(emailDispatcher)
	if handler.emailDispatcher != emailDispatcher {
		t.Fatal("expected EmailDispatcher to be set")
	}

	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{}
	}, cryptoKeyManager)
	handler.SetSMSDispatcher(smsDispatcher)
	if handler.smsDispatcher != smsDispatcher {
		t.Fatal("expected SMSDispatcher to be set")
	}

	serviceAccountManager := core.NewServiceAccountManager(nil)
	handler.SetServiceAccountManager(serviceAccountManager)
	if handler.serviceAccountManager != serviceAccountManager {
		t.Fatal("expected ServiceAccountManager to be set")
	}

	eventBus := core.NewEventBus(nil, cryptoKeyManager)
	defer eventBus.Close()
	handler.SetEventBus(eventBus)
	if handler.eventBus != eventBus {
		t.Fatal("expected EventBus to be set")
	}

	totpManager := handler.GetTOTPManager()
	if totpManager == nil {
		t.Fatal("expected non-nil TOTPManager")
	}

	// Test NewHandler fallback when MFA.Issuer is empty
	emptyIssuerConfigManager := NewConfigManager(nil, cryptoKeyManager)
	emptyIssuerConfigManager.rwMutex.Lock()
	emptyIssuerConfigManager.config.MFA.Issuer = ""
	emptyIssuerConfigManager.rwMutex.Unlock()
	emptyHandler := NewHandler(nil, emptyIssuerConfigManager, cryptoKeyManager)
	if emptyHandler == nil {
		t.Fatal("expected non-nil handler with empty issuer")
	}
}

func TestAuthHandlerCookiesAndHelpersUnit(t *testing.T) {
	responseRecorder := httptest.NewRecorder()
	expiresAt := time.Now().Add(time.Hour)

	// SetSessionCookie HTTPS
	core.SetSessionCookie(responseRecorder, AuthSessionCookieName, AuthSessionInsecureCookieName, "mock-refresh-token", expiresAt, true)
	cookies := responseRecorder.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != AuthSessionCookieName {
		t.Fatalf("expected cookie %s, got: %+v", AuthSessionCookieName, cookies)
	}

	// SetSessionCookie HTTP
	insecureResponseRecorder := httptest.NewRecorder()
	core.SetSessionCookie(insecureResponseRecorder, AuthSessionCookieName, AuthSessionInsecureCookieName, "mock-insecure-token", expiresAt, false)
	insecureCookies := insecureResponseRecorder.Result().Cookies()
	if len(insecureCookies) == 0 || insecureCookies[0].Name != AuthSessionInsecureCookieName {
		t.Fatalf("expected cookie %s, got: %+v", AuthSessionInsecureCookieName, insecureCookies)
	}

	// ClearSessionCookie
	clearResponseRecorder := httptest.NewRecorder()
	core.ClearSessionCookie(clearResponseRecorder, AuthSessionCookieName, AuthSessionInsecureCookieName, true)
	clearedCookies := clearResponseRecorder.Result().Cookies()
	if len(clearedCookies) < 2 {
		t.Fatalf("expected cleared cookies, got: %+v", clearedCookies)
	}

	// nilIfEmpty
	if nilIfEmpty("") != nil {
		t.Fatal("expected nil on empty string")
	}
	if *nilIfEmpty("val") != "val" {
		t.Fatal("expected non-nil value")
	}
}

func TestAuthHandlerDeliveryReadinessUnit(t *testing.T) {
	cryptoKeyManager, _ := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)

	// Unconfigured email dispatcher
	if handler.assertEmailDeliveryReady(responseRecorder, request) {
		t.Fatal("expected assertEmailDeliveryReady to return false when unconfigured")
	}
	if responseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got: %d", responseRecorder.Code)
	}

	// Unconfigured SMS dispatcher
	smsResponseRecorder := httptest.NewRecorder()
	if handler.assertSMSDeliveryReady(smsResponseRecorder, request) {
		t.Fatal("expected assertSMSDeliveryReady to return false when unconfigured")
	}
	if smsResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got: %d", smsResponseRecorder.Code)
	}

	// Configured email dispatcher
	webhookDriver := "webhook"
	configuredEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: EmailDispatcherWebhookConfig{URL: "https://example.com/webhook"},
		}
	}, cryptoKeyManager)
	handler.SetEmailDispatcher(configuredEmailDispatcher)
	if !handler.assertEmailDeliveryReady(responseRecorder, request) {
		t.Fatal("expected assertEmailDeliveryReady to return true when configured")
	}

	// Configured SMS dispatcher
	configuredSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: SMSDispatcherWebhookConfig{URL: "https://example.com/sms"},
		}
	}, cryptoKeyManager)
	handler.SetSMSDispatcher(configuredSMSDispatcher)
	if !handler.assertSMSDeliveryReady(smsResponseRecorder, request) {
		t.Fatal("expected assertSMSDeliveryReady to return true when configured")
	}
}

func TestAuthHandlerAuthenticateUserAndSessionUnit(t *testing.T) {
	cryptoKeyManager, _ := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)

	// authenticateSessionRequest: no token
	noTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if _, _, _, err := handler.authenticateSessionRequest(noTokenRequest); err == nil {
		t.Fatal("expected error on missing token")
	}

	// authenticateSessionRequest: invalid token
	invalidTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	invalidTokenRequest.Header.Set("Authorization", "Bearer invalid-jwt")
	if _, _, _, err := handler.authenticateSessionRequest(invalidTokenRequest); err == nil {
		t.Fatal("expected error on invalid jwt")
	}

	// authenticateSessionRequest: valid token
	testUserID := uuid.NewV7().String()
	validAccessToken, err := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject: testUserID,
		Email:   "test@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	validRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	validRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	validRequest.Header.Set("X-Refresh-Token", "refresh-token-xyz")
	validRequest.Header.Set("X-Session-ID", "session-id-123")

	subject, refreshHash, sessionID, err := handler.authenticateSessionRequest(validRequest)
	if err != nil || subject != testUserID || refreshHash == "" || sessionID != "session-id-123" {
		t.Fatalf("unexpected authenticateSessionRequest result: %s, %s, %s, %v", subject, refreshHash, sessionID, err)
	}

	// authenticateUser: via valid access token
	authedUser, err := handler.authenticateUser(validRequest)
	if err != nil || authedUser != testUserID {
		t.Fatalf("expected authedUser %s, got %s (err: %v)", testUserID, authedUser, err)
	}

	// authenticateUser: no token, no refresh cookie -> unauthorized
	emptyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if _, authErr := handler.authenticateUser(emptyRequest); authErr == nil {
		t.Fatal("expected unauthorized error")
	}

	// authenticateSessionRequest: nil handler
	var nilHandler *Handler
	if _, _, _, nilErr := nilHandler.authenticateSessionRequest(validRequest); nilErr == nil {
		t.Fatal("expected error with nil handler")
	}

	// authenticateUser: nil handler
	if _, nilErr := nilHandler.authenticateUser(validRequest); nilErr == nil {
		t.Fatal("expected error with nil handler")
	}

	// authenticateSessionRequest with cookie
	cookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	cookieRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	cookieRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: "cookie-refresh-token"})
	cookieRequest.Header.Set("X-Session-ID", "sess-1")
	if _, hash, _, cookieErr := handler.authenticateSessionRequest(cookieRequest); cookieErr != nil || hash == "" {
		t.Fatalf("expected session request via secure cookie: %v", cookieErr)
	}

	// authenticateSessionRequest with insecure cookie
	insecureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	insecureCookieRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	insecureCookieRequest.AddCookie(&http.Cookie{Name: AuthSessionInsecureCookieName, Value: "insecure-refresh-token"})
	if _, hash, _, insecureErr := handler.authenticateSessionRequest(insecureCookieRequest); insecureErr != nil || hash == "" {
		t.Fatalf("expected session request via insecure cookie: %v", insecureErr)
	}

	// authenticateSessionRequest with X-Session-Token
	sessionHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	sessionHeaderRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	sessionHeaderRequest.Header.Set("X-Session-Token", "header-session-token")
	if _, hash, _, headerErr := handler.authenticateSessionRequest(sessionHeaderRequest); headerErr != nil || hash == "" {
		t.Fatalf("expected session request via session header: %v", headerErr)
	}

	// authenticateUser with cookie (no access token)
	authCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	authCookieRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: "refresh-cookie"})
	_, _ = handler.authenticateUser(authCookieRequest)

	// authenticateUser with insecure cookie (no access token)
	authInsecureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	authInsecureRequest.AddCookie(&http.Cookie{Name: AuthSessionInsecureCookieName, Value: "insecure-refresh-cookie"})
	_, _ = handler.authenticateUser(authInsecureRequest)

	// authenticateUser with X-Refresh-Token (no access token)
	authRefreshRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	authRefreshRequest.Header.Set("X-Refresh-Token", "token-header")
	_, _ = handler.authenticateUser(authRefreshRequest)

	// authenticateUser with X-Session-Token (no access token)
	authSessionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	authSessionRequest.Header.Set("X-Session-Token", "session-header")
	_, _ = handler.authenticateUser(authSessionRequest)

	// extractClaimsOptional
	claims := handler.extractClaimsOptional(validRequest)
	if claims == nil || claims.Subject != testUserID {
		t.Fatalf("expected claims subject %s, got: %+v", testUserID, claims)
	}
	if handler.extractClaimsOptional(emptyRequest) != nil {
		t.Fatal("expected nil claims on empty request")
	}
	invalidOptionalClaimsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	invalidOptionalClaimsRequest.Header.Set("Authorization", "Bearer invalid-token-string")
	if handler.extractClaimsOptional(invalidOptionalClaimsRequest) != nil {
		t.Fatal("expected nil claims on invalid token")
	}

	// resolveCustomClaims with nil db
	customClaims := handler.resolveCustomClaims(context.Background(), testUserID)
	if customClaims != nil {
		t.Fatalf("expected nil custom claims with nil db, got: %+v", customClaims)
	}

	// resolveAnonymousCaller with nil db
	resolvedAnonymousUserRecord, resolveAnonymousCallerErr := handler.resolveAnonymousCaller(validRequest)
	if resolvedAnonymousUserRecord != nil || !errors.Is(resolveAnonymousCallerErr, ErrAnonymousSessionNotFound) {
		t.Fatalf("expected ErrAnonymousSessionNotFound with nil db, got: %+v (err: %v)", resolvedAnonymousUserRecord, resolveAnonymousCallerErr)
	}
}

func TestAuthHandlerIssueSessionResponseUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	configManager.rwMutex.Lock()
	configManager.config.Sessions.RefreshTokenExpirySeconds = -1
	configManager.config.Cache.FastPathSessionsEnabled = true
	configManager.config.Cache.SessionTTLSeconds = -1
	configManager.rwMutex.Unlock()

	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	testPhone := "+1234567890"
	testEmail := "user@example.com"
	userRecord := UserRecord{
		ID:    uuid.NewV7().String(),
		Role:  "authenticated",
		Email: &testEmail,
		Phone: &testPhone,
	}

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	handler.issueSessionResponse(responseRecorder, request, userRecord)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", responseRecorder.Code)
	}

	// KVStore Set error branch
	testKVStore.setErr = errors.New("simulated kv store set error")
	kvErrResponseRecorder := httptest.NewRecorder()
	handler.issueSessionResponse(kvErrResponseRecorder, request, userRecord)
	if kvErrResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 even if fast path cache fails, got: %d", kvErrResponseRecorder.Code)
	}
	testKVStore.setErr = nil
}
