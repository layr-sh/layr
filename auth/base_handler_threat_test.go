package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/threat"
	"layr.sh/core"
)

type threatMockHTTPClient struct {
	doFunc func(request *http.Request) (*http.Response, error)
}

func (client *threatMockHTTPClient) Do(request *http.Request) (*http.Response, error) {
	if client.doFunc != nil {
		return client.doFunc(request)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
		Header:     make(http.Header),
	}, nil
}

func TestAuthBaseHandlerThreatCheckCaptchaUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager
	testKVStore := kernel.KVStore()
	cryptoKeyManager := kernel.CryptoKeyManager()
	mockClient := &threatMockHTTPClient{}
	baseHandler.httpClient = mockClient

	ctx := context.Background()

	// 1. Bot protection disabled -> returns true
	disabledConfig := DefaultConfig()
	disabledConfig.Threat.BotProtection.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	disabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	disabledResponseRecorder := httptest.NewRecorder()
	if !baseHandler.checkCaptcha(disabledResponseRecorder, disabledRequest, "192.168.1.1", "", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return true when bot protection is disabled")
	}

	// 2. Adaptive mode, failed attempts below threshold -> returns true
	adaptiveConfig := DefaultConfig()
	adaptiveConfig.Threat.BotProtection.Enabled = true
	adaptiveConfig.Threat.BotProtection.Mode = "adaptive"
	adaptiveConfig.Threat.BotProtection.AdaptiveFailedAttempts = 5
	configManager.SetMemoryConfig(adaptiveConfig)

	adaptiveAllowedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	adaptiveAllowedResponseRecorder := httptest.NewRecorder()
	if !baseHandler.checkCaptcha(adaptiveAllowedResponseRecorder, adaptiveAllowedRequest, "192.168.1.2", "", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return true when failed attempts < threshold")
	}

	// 3. Adaptive mode, failed attempts >= threshold with empty token -> returns false (400)
	// Also test with AdaptiveFailedAttempts <= 0 to trigger the fallback threshold branch
	adaptiveConfig.Threat.BotProtection.AdaptiveFailedAttempts = 0
	configManager.SetMemoryConfig(adaptiveConfig)
	for index := 0; index < 5; index++ {
		_, _ = threat.RecordFailedAttempt(ctx, testKVStore, "192.168.1.3", 0)
	}

	adaptiveTriggeredRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	adaptiveTriggeredResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(adaptiveTriggeredResponseRecorder, adaptiveTriggeredRequest, "192.168.1.3", "", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false when adaptive challenge triggered without token")
	}
	if adaptiveTriggeredResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d", adaptiveTriggeredResponseRecorder.Code)
	}

	// 4. Always mode with empty token and event bus configured -> publishes event and returns false (400)

	alwaysConfig := DefaultConfig()
	alwaysConfig.Threat.BotProtection.Enabled = true
	alwaysConfig.Threat.BotProtection.Mode = "always"
	alwaysConfig.Threat.BotProtection.Provider = "turnstile"
	configManager.SetMemoryConfig(alwaysConfig)

	emptyTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	emptyTokenResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(emptyTokenResponseRecorder, emptyTokenRequest, "192.168.1.4", "   ", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false with empty token")
	}
	if emptyTokenResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d", emptyTokenResponseRecorder.Code)
	}

	// 5. Secret decryption error or empty decrypted secret -> returns false (500)
	alwaysConfig.Threat.BotProtection.SecretKey = "invalid-corrupted-prefix"
	configManager.SetMemoryConfig(alwaysConfig)

	decryptFailRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	decryptFailResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(decryptFailResponseRecorder, decryptFailRequest, "192.168.1.5", "token-123", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false when secret decryption fails")
	}
	if decryptFailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got: %d", decryptFailResponseRecorder.Code)
	}

	encryptedEmptySecret, _ := cryptoKeyManager.EncryptField([]byte(""))
	alwaysConfig.Threat.BotProtection.SecretKey = encryptedEmptySecret
	configManager.SetMemoryConfig(alwaysConfig)

	emptySecretRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	emptySecretResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(emptySecretResponseRecorder, emptySecretRequest, "192.168.1.5", "token-123", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false when secret decrypts to empty")
	}
	if emptySecretResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got: %d", emptySecretResponseRecorder.Code)
	}

	// 6. Secret decrypted, but invalid provider -> NewCaptchaVerifier error -> returns false (500)
	encryptedValidSecret, _ := cryptoKeyManager.EncryptField([]byte("real-secret"))
	alwaysConfig.Threat.BotProtection.SecretKey = encryptedValidSecret
	alwaysConfig.Threat.BotProtection.Provider = "unsupported-provider"
	configManager.SetMemoryConfig(alwaysConfig)

	invalidProviderRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	invalidProviderResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(invalidProviderResponseRecorder, invalidProviderRequest, "192.168.1.6", "token-123", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false with unsupported provider")
	}
	if invalidProviderResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got: %d", invalidProviderResponseRecorder.Code)
	}

	// 7. Valid provider, but verification fails -> returns false (400) and publishes event
	alwaysConfig.Threat.BotProtection.Provider = "turnstile"
	configManager.SetMemoryConfig(alwaysConfig)
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":false}`)),
			Header:     make(http.Header),
		}, nil
	}

	verifyFailRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	verifyFailResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkCaptcha(verifyFailResponseRecorder, verifyFailRequest, "192.168.1.7", "invalid-token", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return false when verify returns false")
	}
	if verifyFailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d", verifyFailResponseRecorder.Code)
	}

	// 8. Verification succeeds -> returns true
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			Header:     make(http.Header),
		}, nil
	}

	verifySuccessRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", nil)
	verifySuccessResponseRecorder := httptest.NewRecorder()
	if !baseHandler.checkCaptcha(verifySuccessResponseRecorder, verifySuccessRequest, "192.168.1.8", "valid-token", "/v1/auth/sign-in") {
		t.Fatal("expected checkCaptcha to return true on successful verification")
	}
}

func TestAuthBaseHandlerThreatCheckPasswordBreachUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager
	mockClient := &threatMockHTTPClient{}
	baseHandler.httpClient = mockClient

	ctx := context.Background()

	// 1. Breach check disabled -> returns true
	disabledConfig := DefaultConfig()
	disabledConfig.Password.BreachCheck.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	breachDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", nil)
	breachDisabledResponseRecorder := httptest.NewRecorder()
	if !baseHandler.checkPasswordBreach(breachDisabledResponseRecorder, breachDisabledRequest, "anyPassword", "test@example.com") {
		t.Fatal("expected checkPasswordBreach to return true when disabled")
	}

	// 2. Breach check enabled, network error when FailOpen is false -> returns false (503)
	breachConfig := DefaultConfig()
	breachConfig.Password.BreachCheck.Enabled = true
	breachConfig.Password.BreachCheck.FailOpen = false
	configManager.SetMemoryConfig(breachConfig)
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network timeout")
	}

	breachErrorRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", nil)
	breachErrorResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkPasswordBreach(breachErrorResponseRecorder, breachErrorRequest, "password123", "test@example.com") {
		t.Fatal("expected checkPasswordBreach to return false on network failure with failOpen=false")
	}
	if breachErrorResponseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got: %d", breachErrorResponseRecorder.Code)
	}

	// 3. Password breached -> returns false (400) and publishes event

	// SHA-1 of "password" is 5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8
	// prefix: 5BAA6, suffix: 1E4C9B93F3F0682250B6CF8331B7EE68FD8
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1E4C9B93F3F0682250B6CF8331B7EE68FD8:3861493\r\nOTHER:1\r\n")),
			Header:     make(http.Header),
		}, nil
	}

	breachedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", nil)
	breachedResponseRecorder := httptest.NewRecorder()
	if baseHandler.checkPasswordBreach(breachedResponseRecorder, breachedRequest, "password", "victim@example.com") {
		t.Fatal("expected checkPasswordBreach to return false for breached password")
	}
	if breachedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d", breachedResponseRecorder.Code)
	}

	// 4. Password not breached -> returns true
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("DIFFERENT_HASH_SUFFIX:1\r\n")),
			Header:     make(http.Header),
		}, nil
	}

	safePasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", nil)
	safePasswordResponseRecorder := httptest.NewRecorder()
	if !baseHandler.checkPasswordBreach(safePasswordResponseRecorder, safePasswordRequest, "unbreachedPassword123!", "user@example.com") {
		t.Fatal("expected checkPasswordBreach to return true for safe password")
	}
}
