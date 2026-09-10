package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthDefaultConfigUnit(t *testing.T) {
	authConfig := DefaultConfig()

	if !authConfig.Password.Enabled {
		t.Fatal("expected password method to be enabled by default")
	}
	if authConfig.Password.MinLength != 8 {
		t.Fatalf("expected min length 8, got %d", authConfig.Password.MinLength)
	}
	if authConfig.Sessions.AccessTokenExpirySeconds != 900 {
		t.Fatalf("expected access token TTL 900, got %d", authConfig.Sessions.AccessTokenExpirySeconds)
	}
	if authConfig.Sessions.RefreshTokenExpirySeconds != 2592000 {
		t.Fatalf("expected refresh token TTL 2592000, got %d", authConfig.Sessions.RefreshTokenExpirySeconds)
	}
	if !authConfig.RateLimiting.Enabled {
		t.Fatal("expected rate limiting enabled by default")
	}
	if authConfig.RateLimiting.MaxSigninAttempts != 5 {
		t.Fatalf("expected max 5 signin attempts, got %d", authConfig.RateLimiting.MaxSigninAttempts)
	}
	if !authConfig.Cache.FastPathSessionsEnabled {
		t.Fatal("expected cache fast path enabled")
	}

	if authConfig.EmailDispatcher.Driver != nil {
		t.Fatalf("expected Email driver to be nil (unconfigured) in DefaultConfig, got: %v", *authConfig.EmailDispatcher.Driver)
	}
	if authConfig.EmailDispatcher.SenderName != "" {
		t.Fatalf("expected default sender name to be empty in default config, got: %s", authConfig.EmailDispatcher.SenderName)
	}
	if authConfig.EmailDispatcher.Templates.EmailVerification.Subject == "" || authConfig.EmailDispatcher.Templates.PasswordReset.Subject == "" || authConfig.EmailDispatcher.Templates.SignInOTP.Subject == "" {
		t.Fatalf("expected Email default templates to be populated, got: %+v", authConfig.EmailDispatcher.Templates)
	}

	if authConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected SMS driver to be nil (unconfigured) in DefaultConfig, got: %v", *authConfig.SMSDispatcher.Driver)
	}
	if authConfig.SMSDispatcher.Templates.PhoneVerification.Text == "" || authConfig.SMSDispatcher.Templates.SignInOTP.Text == "" || authConfig.SMSDispatcher.Templates.PasswordReset.Text == "" {
		t.Fatalf("expected SMS default templates to be populated, got: %+v", authConfig.SMSDispatcher.Templates)
	}

	// Test template fallback in ConfigManager
	configManager := NewConfigManager(nil, nil)

	emptyTemplateConfig := authConfig
	emptyTemplateConfig.EmailDispatcher.Templates = EmailDispatcherTemplatesConfig{}
	emptyTemplateConfig.SMSDispatcher.Templates = SMSDispatcherTemplatesConfig{}
	configManager.Set(emptyTemplateConfig)
	if configManager.Get().EmailDispatcher.Templates.EmailVerification.Subject == "" || configManager.Get().SMSDispatcher.Templates.PhoneVerification.Text == "" || configManager.Get().SMSDispatcher.Templates.PasswordReset.Text == "" {
		t.Fatal("expected Set to restore default templates when empty")
	}
}

func TestAuthConfigManagerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	configManager.SetServiceAccountManager(nil)
	configManager.SetEventBus(nil)

	// Test Load and Save on nil pool (should return nil without panic)
	if loadErr := configManager.Load(context.Background()); loadErr != nil {
		t.Fatalf("expected nil error on nil pool Load: %v", loadErr)
	}
	if saveErr := configManager.Save(context.Background(), DefaultConfig()); saveErr != nil {
		t.Fatalf("expected nil error on nil pool Save: %v", saveErr)
	}

	initialConfig := configManager.Get()
	if initialConfig.Password.MinLength != 8 {
		t.Fatalf("expected default min length 8, got: %d", initialConfig.Password.MinLength)
	}

	// Test Set with zero/negative fallbacks
	var emptyConfig Config
	emptyConfig.RateLimiting.Enabled = false
	emptyConfig.RateLimiting.MaxSigninAttempts = 0
	emptyConfig.Cache.FastPathSessionsEnabled = false
	emptyConfig.Cache.SessionTTLSeconds = 0
	configManager.Set(emptyConfig)

	fallbackConfig := configManager.Get()
	if fallbackConfig.Sessions.AccessTokenExpirySeconds != 900 || fallbackConfig.Sessions.RefreshTokenExpirySeconds != 2592000 {
		t.Fatalf("expected fallback token TTLs, got: %+v", fallbackConfig.Sessions)
	}
	if fallbackConfig.Password.MinLength != 8 || fallbackConfig.EmailOTP.TokenExpiryMinutes != 15 || fallbackConfig.SMSOTP.TokenExpiryMinutes != 15 {
		t.Fatalf("expected fallback auth config, got: %+v", fallbackConfig)
	}
	if fallbackConfig.RateLimiting.MaxSigninAttempts != 5 || fallbackConfig.Cache.SessionTTLSeconds != 900 {
		t.Fatalf("expected fallback rate limiting and cache, got: %+v", fallbackConfig)
	}

	// Test GetUnencrypted secret projection
	secretPassword := "super-secret-smtp-password"
	encryptedPassword, err := cryptoKeyManager.EncryptField([]byte(secretPassword))
	if err != nil {
		t.Fatalf("failed to encrypt password: %v", err)
	}

	driverSMTP := "smtp"
	withClientSecretConfig := DefaultConfig()
	withClientSecretConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver: &driverSMTP,
		SMTP: EmailDispatcherSMTPConfig{
			Password: encryptedPassword,
		},
	}
	withClientSecretConfig.OAuthProviders["google"] = OAuthProviderConfig{
		Enabled:      true,
		ClientID:     "google-client-id",
		ClientSecret: encryptedPassword,
	}
	configManager.Set(withClientSecretConfig)

	unencryptedConfig := configManager.GetUnencrypted()
	if unencryptedConfig.EmailDispatcher.SMTP.Password != "" || !unencryptedConfig.EmailDispatcher.SMTP.PasswordConfigured {
		t.Fatalf("expected unencrypted SMTP password to be stripped and PasswordConfigured=true: %+v", unencryptedConfig.EmailDispatcher)
	}
	googleOAuthProviderConfig := unencryptedConfig.OAuthProviders["google"]
	if googleOAuthProviderConfig.ClientSecret != "" || !googleOAuthProviderConfig.ClientSecretConfigured {
		t.Fatalf("expected unencrypted OAuth secret to be stripped and ClientSecretConfigured=true: %+v", googleOAuthProviderConfig)
	}

	// Test DecryptSecret
	decryptedSecret, err := configManager.DecryptSecret(encryptedPassword)
	if err != nil || decryptedSecret != secretPassword {
		t.Fatalf("expected decrypted secret %s, got: %s (err: %v)", secretPassword, decryptedSecret, err)
	}
	emptyDecrypted, err := configManager.DecryptSecret("")
	if err != nil || emptyDecrypted != "" {
		t.Fatalf("expected empty secret for empty input, got: %s", emptyDecrypted)
	}
	if _, err := configManager.DecryptSecret("invalid-encryption-format"); err == nil {
		t.Fatal("expected error on invalid encrypted format")
	}

	// Test DecryptSecret with nil key manager
	nilKeyManagerConfigManager := NewConfigManager(nil, nil)
	if _, err := nilKeyManagerConfigManager.DecryptSecret("some-value"); err == nil {
		t.Fatal("expected error decrypting secret with nil key manager")
	}

	// Test checkScope with invalid secret key
	serviceAccountManager := core.NewServiceAccountManager(nil)
	configManager.SetServiceAccountManager(serviceAccountManager)
	invalidKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/_/auth/config", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid-sec-key")
	if configManager.checkScope(invalidKeyRequest, "auth:config.read") {
		t.Fatal("expected checkScope to fail on invalid secret key")
	}

	// Test checkScope forbidden on HandleGetConfig
	forbiddenGetResponseRecorder := httptest.NewRecorder()
	configManager.HandleGetConfig(forbiddenGetResponseRecorder, invalidKeyRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on HandleGetConfig, got: %d", forbiddenGetResponseRecorder.Code)
	}

	// Test checkScope forbidden on HandlePutConfig
	forbiddenPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(forbiddenPutResponseRecorder, invalidKeyRequest)
	if forbiddenPutResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on HandlePutConfig, got: %d", forbiddenPutResponseRecorder.Code)
	}

	// Test HandlePutConfig success and event bus publish
	eventBus := core.NewEventBus(nil, cryptoKeyManager)
	defer eventBus.Close()
	configManager.SetEventBus(eventBus)

	validPutBody := `{"password":{"enabled":true,"min_length":10},"smtp":{"password":"new-smtp-password"}}`
	validPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(validPutBody))
	validPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(validPutResponseRecorder, validPutRequest)
	if validPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig, got: %d (body: %s)", validPutResponseRecorder.Code, validPutResponseRecorder.Body.String())
	}
	if configManager.Get().Password.MinLength != 10 {
		t.Fatalf("expected updated min length 10, got: %d", configManager.Get().Password.MinLength)
	}

	// Test HandlePutConfig invalid JSON body -> 400
	invalidJSONPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(`{invalid-json`))
	invalidJSONPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(invalidJSONPutResponseRecorder, invalidJSONPutRequest)
	if invalidJSONPutResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid JSON in HandlePutConfig, got: %d", invalidJSONPutResponseRecorder.Code)
	}

	// Test HandlePutConfig with OAuth provider secret update
	oauthPutBody := `{"oauth_providers":{"google":{"enabled":true,"client_id":"new-client-id","client_secret":"new-plaintext-secret"}}}`
	oauthPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(oauthPutBody))
	oauthPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(oauthPutResponseRecorder, oauthPutRequest)
	if oauthPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with OAuth, got: %d", oauthPutResponseRecorder.Code)
	}
	if configManager.Get().OAuthProviders["google"].ClientID != "new-client-id" {
		t.Fatalf("expected updated OAuth client ID, got: %+v", configManager.Get().OAuthProviders["google"])
	}

	// Test HandlePutConfig preserving OAuth secret when empty
	preserveOAuthBody := `{"oauth_providers":{"google":{"enabled":true,"client_id":"new-client-id","client_secret":""}}}`
	preserveOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(preserveOAuthBody))
	preserveOAuthResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(preserveOAuthResponseRecorder, preserveOAuthRequest)
	if preserveOAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig preserving OAuth secret, got: %d", preserveOAuthResponseRecorder.Code)
	}

	// Test HandlePutConfig with null oauth_providers preserving current
	nullOAuthBody := `{"oauth_providers":null}`
	nullOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(nullOAuthBody))
	nullOAuthResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(nullOAuthResponseRecorder, nullOAuthRequest)
	if nullOAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with null oauth_providers, got: %d", nullOAuthResponseRecorder.Code)
	}

	// Test HandlePutConfig with null oidc.clients preserving current
	nullOIDCBody := `{"oidc":{"clients":null}}`
	nullOIDCRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(nullOIDCBody))
	nullOIDCResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(nullOIDCResponseRecorder, nullOIDCRequest)
	if nullOIDCResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with null oidc.clients, got: %d", nullOIDCResponseRecorder.Code)
	}

	// Test HandlePutConfig with Email and SMS plaintext secrets
	emailSMSPutBody := `{
		"email_dispatcher": {
			"driver": "smtp",
			"smtp": {"host": "smtp.test.com", "password": "new-smtp-password"},
			"webhook": {"url": "https://test.com/email", "signing_secret": "new-email-secret"}
		},
		"sms_dispatcher": {
			"driver": "twilio",
			"twilio": {"account_sid": "AC999", "auth_token": "new-auth-token"},
			"webhook": {"url": "https://test.com/sms", "signing_secret": "new-sms-secret"}
		}
	}`
	emailSMSPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(emailSMSPutBody))
	emailSMSPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(emailSMSPutResponseRecorder, emailSMSPutRequest)
	if emailSMSPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with Email and SMS, got: %d", emailSMSPutResponseRecorder.Code)
	}

	savedConfig := configManager.Get()
	if !strings.HasPrefix(savedConfig.EmailDispatcher.SMTP.Password, "enc:v1:") {
		t.Fatalf("expected encrypted smtp password, got: %s", savedConfig.EmailDispatcher.SMTP.Password)
	}
	if !strings.HasPrefix(savedConfig.EmailDispatcher.Webhook.SigningSecret, "enc:v1:") {
		t.Fatalf("expected encrypted email webhook signing secret, got: %s", savedConfig.EmailDispatcher.Webhook.SigningSecret)
	}
	if !strings.HasPrefix(savedConfig.SMSDispatcher.Twilio.AuthToken, "enc:v1:") {
		t.Fatalf("expected encrypted twilio auth token, got: %s", savedConfig.SMSDispatcher.Twilio.AuthToken)
	}
	if !strings.HasPrefix(savedConfig.SMSDispatcher.Webhook.SigningSecret, "enc:v1:") {
		t.Fatalf("expected encrypted sms webhook signing secret, got: %s", savedConfig.SMSDispatcher.Webhook.SigningSecret)
	}

	// Test GetUnencrypted zero-decryption projection for Email and SMS
	unencryptedConfig = configManager.GetUnencrypted()
	if !unencryptedConfig.EmailDispatcher.SMTP.PasswordConfigured || !unencryptedConfig.EmailDispatcher.Webhook.SigningSecretConfigured {
		t.Fatalf("expected email secrets configured in GetUnencrypted, got: %+v", unencryptedConfig.EmailDispatcher)
	}
	if !unencryptedConfig.SMSDispatcher.Twilio.AuthTokenConfigured || !unencryptedConfig.SMSDispatcher.Webhook.SigningSecretConfigured {
		t.Fatalf("expected sms secrets configured in GetUnencrypted, got: %+v", unencryptedConfig.SMSDispatcher)
	}

	// Test HandlePutConfig with empty secrets preserving existing secrets
	preserveBody := `{
		"email_dispatcher": {
			"driver": "smtp",
			"smtp": {"host": "smtp.test.com", "password": ""},
			"webhook": {"url": "https://test.com/email", "signing_secret": ""}
		},
		"sms_dispatcher": {
			"driver": "twilio",
			"twilio": {"account_sid": "AC999", "auth_token": ""},
			"webhook": {"url": "https://test.com/sms", "signing_secret": ""}
		}
	}`
	preserveRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(preserveBody))
	preserveResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(preserveResponseRecorder, preserveRequest)
	if preserveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig preserving secrets, got: %d", preserveResponseRecorder.Code)
	}

	preservedConfig := configManager.Get()
	if preservedConfig.EmailDispatcher.SMTP.Password != savedConfig.EmailDispatcher.SMTP.Password {
		t.Fatalf("expected smtp password to be preserved")
	}
	if preservedConfig.SMSDispatcher.Twilio.AuthToken != savedConfig.SMSDispatcher.Twilio.AuthToken {
		t.Fatalf("expected twilio auth token to be preserved")
	}

	// Test setting email and sms driver to null again by console user after set
	nullDriverBody := `{
		"email_dispatcher": {
			"driver": null
		},
		"sms_dispatcher": {
			"driver": null
		}
	}`
	nullDriverRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(nullDriverBody))
	nullDriverResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(nullDriverResponseRecorder, nullDriverRequest)
	if nullDriverResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with null driver, got: %d", nullDriverResponseRecorder.Code)
	}

	nullDriverConfig := configManager.Get()
	if nullDriverConfig.EmailDispatcher.Driver != nil {
		t.Fatalf("expected email driver to be nil after console user resets to null, got: %v", *nullDriverConfig.EmailDispatcher.Driver)
	}
	if nullDriverConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected sms driver to be nil after console user resets to null, got: %v", *nullDriverConfig.SMSDispatcher.Driver)
	}

	// Verify unencrypted outputs "driver": null
	nullDriverEncryptedConfig := configManager.GetUnencrypted()
	if nullDriverEncryptedConfig.EmailDispatcher.Driver != nil || nullDriverEncryptedConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected unencrypted email and sms driver to be nil, got: %v, %v", nullDriverEncryptedConfig.EmailDispatcher.Driver, nullDriverEncryptedConfig.SMSDispatcher.Driver)
	}
	if !strings.Contains(nullDriverResponseRecorder.Body.String(), `"driver":null`) {
		t.Fatalf("expected response body to contain null driver, got: %s", nullDriverResponseRecorder.Body.String())
	}

	// Test setting driver to empty string "" also normalizes to nil
	emptyDriverBody := `{
		"email_dispatcher": {
			"driver": ""
		},
		"sms_dispatcher": {
			"driver": ""
		}
	}`
	emptyDriverRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(emptyDriverBody))
	emptyDriverResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(emptyDriverResponseRecorder, emptyDriverRequest)
	if emptyDriverResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with empty string driver, got: %d", emptyDriverResponseRecorder.Code)
	}
	emptyDriverConfig := configManager.Get()
	if emptyDriverConfig.EmailDispatcher.Driver != nil {
		t.Fatalf("expected email driver to be normalized to nil from empty string, got: %v", *emptyDriverConfig.EmailDispatcher.Driver)
	}
	if emptyDriverConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected sms driver to be normalized to nil from empty string, got: %v", *emptyDriverConfig.SMSDispatcher.Driver)
	}

	// Test that email and sms cannot be set to null via HandlePutConfig
	nullConfigBody := `{
		"email_dispatcher": null,
		"sms_dispatcher": null
	}`
	nullConfigRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(nullConfigBody))
	nullConfigResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(nullConfigResponseRecorder, nullConfigRequest)
	if nullConfigResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with null email/sms, got: %d", nullConfigResponseRecorder.Code)
	}
	afterNullConfig := configManager.Get()
	if afterNullConfig.EmailDispatcher.Templates.EmailVerification.Subject == "" {
		t.Fatal("expected email templates to be preserved when payload sends null email")
	}
	if afterNullConfig.SMSDispatcher.Templates.PhoneVerification.Text == "" || afterNullConfig.SMSDispatcher.Templates.PasswordReset.Text == "" {
		t.Fatal("expected sms templates to be preserved when payload sends null sms")
	}

	// Test HandlePutConfig without keyManager
	nilCryptoKeyManagerConfigManager := NewConfigManager(nil, nil)
	plainSecretBody := `{
		"email_dispatcher": {
			"smtp": {"password": "plain-password"},
			"webhook": {"signing_secret": "plain-secret"}
		},
		"sms_dispatcher": {
			"twilio": {"auth_token": "plain-token"},
			"webhook": {"signing_secret": "plain-secret"}
		}
	}`
	plainRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(plainSecretBody))
	plainResponseRecorder := httptest.NewRecorder()
	nilCryptoKeyManagerConfigManager.HandlePutConfig(plainResponseRecorder, plainRequest)
	if plainResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig without keyManager, got: %d", plainResponseRecorder.Code)
	}

	// Test HandlePutConfig rejecting email_otp.enabled without active SMTP -> 422
	unconfiguredOTPPutBody := `{"email_otp":{"enabled":true}}`
	unconfiguredOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(unconfiguredOTPPutBody))
	unconfiguredOTPPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(unconfiguredOTPPutResponseRecorder, unconfiguredOTPPutRequest)
	if unconfiguredOTPPutResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on OTP enable without active SMTP, got: %d (%s)", unconfiguredOTPPutResponseRecorder.Code, unconfiguredOTPPutResponseRecorder.Body.String())
	}
	if !strings.Contains(unconfiguredOTPPutResponseRecorder.Body.String(), "LAYR_AUTH_EMAIL_UNCONFIGURED") {
		t.Fatalf("expected LAYR_AUTH_EMAIL_UNCONFIGURED, got: %s", unconfiguredOTPPutResponseRecorder.Body.String())
	}

	// Test HandlePutConfig rejecting sms_otp.enabled without active SMS provider -> 422
	unconfiguredSMSOTPPutBody := `{"sms_otp":{"enabled":true}}`
	unconfiguredSMSOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(unconfiguredSMSOTPPutBody))
	unconfiguredSMSOTPPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(unconfiguredSMSOTPPutResponseRecorder, unconfiguredSMSOTPPutRequest)
	if unconfiguredSMSOTPPutResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on SMS OTP enable without active SMS provider, got: %d (%s)", unconfiguredSMSOTPPutResponseRecorder.Code, unconfiguredSMSOTPPutResponseRecorder.Body.String())
	}
	if !strings.Contains(unconfiguredSMSOTPPutResponseRecorder.Body.String(), "LAYR_AUTH_SMS_UNCONFIGURED") {
		t.Fatalf("expected LAYR_AUTH_SMS_UNCONFIGURED, got: %s", unconfiguredSMSOTPPutResponseRecorder.Body.String())
	}

	// Test HandlePutConfig enabling OTP with active SMTP -> 200 OK
	configuredOTPPutBody := `{
		"email_otp":{"enabled":true},
		"email_dispatcher":{
			"driver":"smtp",
			"smtp":{"host":"smtp.example.com","port":587,"username":"user","password":"password"}
		}
	}`
	configuredOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(configuredOTPPutBody))
	configuredOTPPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(configuredOTPPutResponseRecorder, configuredOTPPutRequest)
	if configuredOTPPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on OTP enable with active SMTP, got: %d (%s)", configuredOTPPutResponseRecorder.Code, configuredOTPPutResponseRecorder.Body.String())
	}

	// Test HandlePutConfig enabling SMS OTP with active SMS provider -> 200 OK
	configuredSMSOTPPutBody := `{
		"sms_otp":{"enabled":true},
		"sms_dispatcher":{
			"driver":"twilio",
			"twilio":{"account_sid":"AC123","auth_token":"token123","from_number":"+15551234567"}
		}
	}`
	configuredSMSOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(configuredSMSOTPPutBody))
	configuredSMSOTPPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(configuredSMSOTPPutResponseRecorder, configuredSMSOTPPutRequest)
	if configuredSMSOTPPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on SMS OTP enable with active SMS provider, got: %d (%s)", configuredSMSOTPPutResponseRecorder.Code, configuredSMSOTPPutResponseRecorder.Body.String())
	}
}

func TestAuthConfigManagerOIDCAndSignInUIUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)

	// 1. Check defaults
	defaultConfig := configManager.Get()
	if defaultConfig.OIDC.Clients == nil {
		t.Fatal("expected OIDC.Clients slice to be initialized in default config")
	}
	if defaultConfig.OIDC.Enabled {
		t.Fatal("expected OIDC.Enabled to be false by default")
	}

	// 2. Test GetOIDCClient on empty configuration
	if _, found := configManager.GetOIDCClient("non-existent"); found {
		t.Fatal("expected GetOIDCClient to return false for non-existent client")
	}

	// 3. Test HandlePutConfig with OIDC clients and SignInUI
	oidcPutBody := `{
		"oidc": {
			"enabled": true,
			"sign_in_ui": {
				"custom_css": ".auth-card { border-radius: 12px; }",
				"logo_url": "https://example.com/assets/logo.svg"
			},
			"clients": [
				{
					"name": "Web Dashboard",
					"client_id": "web-client-id",
					"client_secret": "raw-confidential-secret",
					"redirect_uris": ["https://app.example.com/callback"],
					"post_sign_out_redirect_uris": ["https://app.example.com/signed-out"],
					"public": false,
					"scopes": ["openid", "profile", "email"]
				},
				{
					"name": "SPA Mobile Client",
					"client_id": "spa-client-id",
					"redirect_uris": ["https://spa.example.com/callback"],
					"public": true,
					"scopes": ["openid", "email"]
				}
			]
		}
	}`

	putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(oidcPutBody))
	putResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(putResponseRecorder, putRequest)

	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig with OIDC config, got: %d (%s)", putResponseRecorder.Code, putResponseRecorder.Body.String())
	}

	savedConfig := configManager.Get()
	if !savedConfig.OIDC.Enabled {
		t.Fatal("expected saved OIDC.Enabled to be true")
	}
	if savedConfig.OIDC.SignInUI.CustomCSS != ".auth-card { border-radius: 12px; }" {
		t.Fatalf("expected saved custom_css, got: %s", savedConfig.OIDC.SignInUI.CustomCSS)
	}
	if savedConfig.OIDC.SignInUI.LogoURL != "https://example.com/assets/logo.svg" {
		t.Fatalf("expected saved logo_url, got: %s", savedConfig.OIDC.SignInUI.LogoURL)
	}

	webOIDCClientConfig, found := configManager.GetOIDCClient("web-client-id")
	if !found {
		t.Fatal("expected GetOIDCClient to find web-client-id")
	}
	if !strings.HasPrefix(webOIDCClientConfig.ClientSecret, "enc:v1:") {
		t.Fatalf("expected web client secret to be envelope-encrypted with enc:v1:, got: %s", webOIDCClientConfig.ClientSecret)
	}
	if webOIDCClientConfig.Public {
		t.Fatal("expected web client to not be public")
	}

	decryptedClientSecret, err := configManager.DecryptSecret(webOIDCClientConfig.ClientSecret)
	if err != nil {
		t.Fatalf("failed to decrypt client secret: %v", err)
	}
	if decryptedClientSecret != "raw-confidential-secret" {
		t.Fatalf("expected decrypted secret 'raw-confidential-secret', got: %s", decryptedClientSecret)
	}

	spaOIDCClientConfig, found := configManager.GetOIDCClient("spa-client-id")
	if !found {
		t.Fatal("expected GetOIDCClient to find spa-client-id")
	}
	if !spaOIDCClientConfig.Public {
		t.Fatal("expected spa client to be public")
	}
	if spaOIDCClientConfig.ClientSecret != "" {
		t.Fatalf("expected empty secret for public client, got: %s", spaOIDCClientConfig.ClientSecret)
	}

	if _, clientFound := configManager.GetOIDCClient("non-existent-client"); clientFound {
		t.Fatal("expected GetOIDCClient to return false for non-matching client ID")
	}

	// 4. Test GetUnencrypted zero-decryption projection
	unencryptedConfig := configManager.GetUnencrypted()
	if unencryptedConfig.OIDC.SignInUI.CustomCSS != ".auth-card { border-radius: 12px; }" {
		t.Fatalf("expected unencrypted custom_css preserved, got: %s", unencryptedConfig.OIDC.SignInUI.CustomCSS)
	}
	if len(unencryptedConfig.OIDC.Clients) != 2 {
		t.Fatalf("expected 2 unencrypted OIDC clients, got: %d", len(unencryptedConfig.OIDC.Clients))
	}
	unencryptedWebOIDCClientConfig := unencryptedConfig.OIDC.Clients[0]
	if unencryptedWebOIDCClientConfig.ClientSecret != "" {
		t.Fatalf("expected stripped client_secret in GetUnencrypted, got: %s", unencryptedWebOIDCClientConfig.ClientSecret)
	}
	if !unencryptedWebOIDCClientConfig.ClientSecretConfigured {
		t.Fatal("expected ClientSecretConfigured=true in GetUnencrypted")
	}

	unencryptedSPAOIDCClientConfig := unencryptedConfig.OIDC.Clients[1]
	if unencryptedSPAOIDCClientConfig.ClientSecret != "" {
		t.Fatalf("expected empty client_secret for public client in GetUnencrypted, got: %s", unencryptedSPAOIDCClientConfig.ClientSecret)
	}
	if unencryptedSPAOIDCClientConfig.ClientSecretConfigured {
		t.Fatal("expected ClientSecretConfigured=false for public client without secret")
	}

	// 5. Test HandlePutConfig preserving secret when empty client_secret is provided
	preserveSecretBody := `{
		"oidc": {
			"enabled": true,
			"clients": [
				{
					"name": "Web Dashboard Renamed",
					"client_id": "web-client-id",
					"client_secret": "",
					"redirect_uris": ["https://app.example.com/callback-updated"],
					"public": false
				}
			]
		}
	}`
	preserveRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(preserveSecretBody))
	preserveResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(preserveResponseRecorder, preserveRequest)

	if preserveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK preserving client secret, got: %d (%s)", preserveResponseRecorder.Code, preserveResponseRecorder.Body.String())
	}

	updatedWebOIDCClientConfig, found := configManager.GetOIDCClient("web-client-id")
	if !found {
		t.Fatal("expected to find updated web client")
	}
	if updatedWebOIDCClientConfig.Name != "Web Dashboard Renamed" {
		t.Fatalf("expected updated client name, got: %s", updatedWebOIDCClientConfig.Name)
	}
	if updatedWebOIDCClientConfig.ClientSecret != webOIDCClientConfig.ClientSecret {
		t.Fatalf("expected client secret to be preserved, got: %s (original: %s)", updatedWebOIDCClientConfig.ClientSecret, webOIDCClientConfig.ClientSecret)
	}
}
