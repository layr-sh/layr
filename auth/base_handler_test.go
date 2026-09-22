package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
)

type failingTestKVDriver struct {
	core.TestInMemoryKVDriver
	setErr error
}

func (driver *failingTestKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	if driver.setErr != nil {
		return driver.setErr
	}
	if err := driver.TestInMemoryKVDriver.Set(ctx, key, value, expiry); err != nil {
		return fmt.Errorf("failed to set in test kv driver: %w", err)
	}
	return nil
}

func TestAuthHandlerInitializationUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler

	if baseHandler == nil {
		t.Fatal("expected non-nil baseHandler")
	}
	if baseHandler.kernel.KVStore() != kernel.KVStore() {
		t.Fatal("expected KVStore to match kernel")
	}
	if baseHandler.kernel.ServiceAccountManager() != kernel.ServiceAccountManager() {
		t.Fatal("expected ServiceAccountManager to match kernel")
	}
	if baseHandler.kernel.EventBus() != kernel.EventBus() {
		t.Fatal("expected EventBus to match kernel")
	}

	emailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig { return nil })
	service.emailDispatcher = emailDispatcher
	if service.emailDispatcher != emailDispatcher {
		t.Fatal("expected EmailDispatcher to be set")
	}

	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return nil })
	service.smsDispatcher = smsDispatcher
	if service.smsDispatcher != smsDispatcher {
		t.Fatal("expected SMSDispatcher to be set")
	}

	totpManager := baseHandler.totpManager
	if totpManager == nil {
		t.Fatal("expected non-nil TOTPManager")
	}

	// Test fallback when MFA.Issuer is empty
	service.configManager.rwMutex.Lock()
	service.configManager.config.MFA.Issuer = ""
	service.configManager.rwMutex.Unlock()
	emptyService := NewService(kernel)
	if emptyService.baseHandler == nil {
		t.Fatal("expected non-nil baseHandler with empty issuer")
	}

	baseHandler.verifyDummyPassword("any-pass")
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
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler

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
	configuredEmailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: EmailDispatcherWebhookConfig{URL: "https://example.com/webhook"},
		}
	})
	service.emailDispatcher = configuredEmailDispatcher
	if !baseHandler.assertEmailDeliveryReady(responseRecorder, request) {
		t.Fatal("expected assertEmailDeliveryReady to return true when configured")
	}

	// Configured SMS dispatcher
	configuredSMSDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  &webhookDriver,
			Webhook: SMSDispatcherWebhookConfig{URL: "https://example.com/sms"},
		}
	})
	service.smsDispatcher = configuredSMSDispatcher
	if !baseHandler.assertSMSDeliveryReady(smsResponseRecorder, request) {
		t.Fatal("expected assertSMSDeliveryReady to return true when configured")
	}
}

func TestAuthHandlerResolveCallerUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler

	testUserID := uuid.NewV7().String()

	// resolveCustomClaims with broken db
	customClaims := baseHandler.resolveCustomClaims(context.Background(), testUserID)
	if customClaims != nil {
		t.Fatalf("expected nil custom claims with broken db, got: %+v", customClaims)
	}

	// resolveAnonymousCaller with broken db
	validRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	resolvedAnonymousUser, resolveAnonymousCallerErr := baseHandler.resolveAnonymousCaller(validRequest)
	if resolvedAnonymousUser != nil || !errors.Is(resolveAnonymousCallerErr, ErrAnonymousSessionNotFound) {
		t.Fatalf("expected ErrAnonymousSessionNotFound with broken db, got: %+v (err: %v)", resolvedAnonymousUser, resolveAnonymousCallerErr)
	}
}

func TestAuthHandlerIssueSessionResponseUnit(t *testing.T) {
	failingDriver := &failingTestKVDriver{TestInMemoryKVDriver: *core.NewTestInMemoryKVDriver()}
	kvStore := core.NewKVStoreFromDriver(failingDriver)

	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations, core.WithKVStore(kvStore))
	service := NewService(kernel)
	baseHandler := service.baseHandler

	service.configManager.rwMutex.Lock()
	service.configManager.config.Sessions.RefreshTokenExpirySeconds = -1
	service.configManager.config.Cache.FastPathSessionsEnabled = true
	service.configManager.config.Cache.SessionTTLSeconds = -1
	service.configManager.rwMutex.Unlock()

	testPhone := "+1234567890"
	testEmail := "user@example.com"
	user := User{
		ID:    uuid.NewV7().String(),
		Role:  "authenticated",
		Email: &testEmail,
		Phone: &testPhone,
	}

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	baseHandler.issueSessionResponse(responseRecorder, request, user)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", responseRecorder.Code)
	}

	// KVStore Set error branch
	failingDriver.setErr = errors.New("simulated kv store set error")
	kvErrResponseRecorder := httptest.NewRecorder()
	baseHandler.issueSessionResponse(kvErrResponseRecorder, request, user)
	if kvErrResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 even if fast path cache fails, got: %d", kvErrResponseRecorder.Code)
	}
	failingDriver.setErr = nil
}
