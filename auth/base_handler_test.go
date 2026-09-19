package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
)

func TestAuthHandlerInitializationUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)
	if baseHandler == nil {
		t.Fatal("expected non-nil baseHandler")
	}

	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)
	if baseHandler.kvStore != testKVStore {
		t.Fatal("expected KVStore to be set")
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{}
	}, cryptoKeyManager)
	baseHandler.SetEmailDispatcher(emailDispatcher)
	if baseHandler.emailDispatcher != emailDispatcher {
		t.Fatal("expected EmailDispatcher to be set")
	}

	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{}
	}, cryptoKeyManager)
	baseHandler.SetSMSDispatcher(smsDispatcher)
	if baseHandler.smsDispatcher != smsDispatcher {
		t.Fatal("expected SMSDispatcher to be set")
	}

	serviceAccountManager := core.NewServiceAccountManager(nil)
	baseHandler.SetServiceAccountManager(serviceAccountManager)
	if baseHandler.serviceAccountManager != serviceAccountManager {
		t.Fatal("expected ServiceAccountManager to be set")
	}

	eventBus := core.NewEventBus(nil, cryptoKeyManager)
	defer eventBus.Close()
	baseHandler.SetEventBus(eventBus)
	if baseHandler.eventBus != eventBus {
		t.Fatal("expected EventBus to be set")
	}

	totpManager := baseHandler.GetTOTPManager()
	if totpManager == nil {
		t.Fatal("expected non-nil TOTPManager")
	}

	// Test NewHandler fallback when MFA.Issuer is empty
	emptyIssuerConfigManager := NewConfigManager(nil, cryptoKeyManager)
	emptyIssuerConfigManager.rwMutex.Lock()
	emptyIssuerConfigManager.config.MFA.Issuer = ""
	emptyIssuerConfigManager.rwMutex.Unlock()
	emptyBaseHandler := NewHandler(nil, emptyIssuerConfigManager, cryptoKeyManager)
	if emptyBaseHandler == nil {
		t.Fatal("expected non-nil baseHandler with empty issuer")
	}
}

func TestAuthHandlerCookiesAndHelpersUnit(t *testing.T) {
	responseRecorder := httptest.NewRecorder()
	expiresAt := time.Now().Add(time.Hour)

	secureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://localhost/", nil)
	secureRequest.TLS = &tls.ConnectionState{}
	insecureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/", nil)

	// SetSessionCookie HTTPS
	core.SetSessionCookie(responseRecorder, secureRequest, "mock-refresh-token", expiresAt)
	cookies := responseRecorder.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != core.SessionCookieNameSecure {
		t.Fatalf("expected cookie %s, got: %+v", core.SessionCookieNameSecure, cookies)
	}

	// SetSessionCookie HTTP
	insecureResponseRecorder := httptest.NewRecorder()
	core.SetSessionCookie(insecureResponseRecorder, insecureRequest, "mock-insecure-token", expiresAt)
	insecureCookies := insecureResponseRecorder.Result().Cookies()
	if len(insecureCookies) == 0 || insecureCookies[0].Name != core.SessionCookieNameInsecure {
		t.Fatalf("expected cookie %s, got: %+v", core.SessionCookieNameInsecure, insecureCookies)
	}

	// ClearSessionCookie
	clearResponseRecorder := httptest.NewRecorder()
	core.ClearSessionCookie(clearResponseRecorder, secureRequest)
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
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)

	// Unconfigured email dispatcher
	if baseHandler.assertEmailDeliveryReady(responseRecorder, request) {
		t.Fatal("expected assertEmailDeliveryReady to return false when unconfigured")
	}
	if responseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got: %d", responseRecorder.Code)
	}

	// Unconfigured SMS dispatcher
	smsResponseRecorder := httptest.NewRecorder()
	if baseHandler.assertSMSDeliveryReady(smsResponseRecorder, request) {
		t.Fatal("expected assertSMSDeliveryReady to return false when unconfigured")
	}
	if smsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got: %d", smsResponseRecorder.Code)
	}

	// Configured email dispatcher
	webhookDriver := "webhook"
	configuredEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: EmailDispatcherWebhookConfig{URL: "https://example.com/webhook"},
		}
	}, cryptoKeyManager)
	baseHandler.SetEmailDispatcher(configuredEmailDispatcher)
	if !baseHandler.assertEmailDeliveryReady(responseRecorder, request) {
		t.Fatal("expected assertEmailDeliveryReady to return true when configured")
	}

	// Configured SMS dispatcher
	configuredSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: SMSDispatcherWebhookConfig{URL: "https://example.com/sms"},
		}
	}, cryptoKeyManager)
	baseHandler.SetSMSDispatcher(configuredSMSDispatcher)
	if !baseHandler.assertSMSDeliveryReady(smsResponseRecorder, request) {
		t.Fatal("expected assertSMSDeliveryReady to return true when configured")
	}
}

func TestAuthHandlerResolveCallerUnit(t *testing.T) {
	cryptoKeyManager, _ := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)

	testUserID := uuid.NewV7().String()

	// resolveCustomClaims with nil db
	customClaims := baseHandler.resolveCustomClaims(context.Background(), testUserID)
	if customClaims != nil {
		t.Fatalf("expected nil custom claims with nil db, got: %+v", customClaims)
	}

	// resolveAnonymousCaller with nil db
	validRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	resolvedAnonymousUserRecord, resolveAnonymousCallerErr := baseHandler.resolveAnonymousCaller(validRequest)
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

	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVDriver := newInMemoryKVDriver()
	testKVStore := core.NewKVStoreFromDriver(testKVDriver)
	baseHandler.SetKVStore(testKVStore)

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
	baseHandler.issueSessionResponse(responseRecorder, request, userRecord)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", responseRecorder.Code)
	}

	// KVStore Set error branch
	testKVDriver.setErr = errors.New("simulated kv store set error")
	kvErrResponseRecorder := httptest.NewRecorder()
	baseHandler.issueSessionResponse(kvErrResponseRecorder, request, userRecord)
	if kvErrResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 even if fast path cache fails, got: %d", kvErrResponseRecorder.Code)
	}
	testKVDriver.setErr = nil
}
