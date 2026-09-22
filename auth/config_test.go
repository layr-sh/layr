package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

type brokenBodyReader struct{}

func (brokenBodyReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read error")
}

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
	if authConfig.RateLimiting.MaxSignInAttempts != 5 {
		t.Fatalf("expected max 5 sign-in attempts, got %d", authConfig.RateLimiting.MaxSignInAttempts)
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
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	configManager := NewConfigManager(kernel)

	emptyTemplateConfig := authConfig
	emptyTemplateConfig.EmailDispatcher.Templates = EmailDispatcherTemplatesConfig{}
	emptyTemplateConfig.SMSDispatcher.Templates = SMSDispatcherTemplatesConfig{}
	configManager.Set(emptyTemplateConfig)
	if configManager.Get().EmailDispatcher.Templates.EmailVerification.Subject == "" || configManager.Get().SMSDispatcher.Templates.PhoneVerification.Text == "" || configManager.Get().SMSDispatcher.Templates.PasswordReset.Text == "" {
		t.Fatal("expected Set to restore default templates when empty")
	}
}

func TestAuthConfigManagerUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	cryptoKeyManager := kernel.CryptoKeyManager()

	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenConfigManager := NewConfigManager(brokenKernel)

	// Test Load and Save on broken pool (should return error)
	if loadErr := brokenConfigManager.Load(context.Background()); loadErr == nil {
		t.Fatal("expected error on broken pool Load")
	}
	if saveErr := brokenConfigManager.Save(context.Background(), DefaultConfig()); saveErr == nil {
		t.Fatal("expected error on broken pool Save")
	}

	configManager := NewConfigManager(kernel)

	initialConfig := configManager.Get()
	if initialConfig.Password.MinLength != 8 {
		t.Fatalf("expected default min length 8, got: %d", initialConfig.Password.MinLength)
	}

	// Test Set with zero/negative fallbacks
	var emptyConfig Config
	emptyConfig.RateLimiting.Enabled = false
	emptyConfig.RateLimiting.MaxSignInAttempts = 0
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
	if fallbackConfig.RateLimiting.MaxSignInAttempts != 5 || fallbackConfig.Cache.SessionTTLSeconds != 900 {
		t.Fatalf("expected fallback rate limiting and cache, got: %+v", fallbackConfig)
	}

	var adaptiveConfig Config
	adaptiveConfig.MFA.Policy = "adaptive"
	configManager.Set(adaptiveConfig)
	adaptiveSavedConfig := configManager.Get()
	if len(adaptiveSavedConfig.MFA.RiskTriggers) != 3 {
		t.Fatalf("expected 3 default risk triggers for adaptive MFA, got: %+v", adaptiveSavedConfig.MFA.RiskTriggers)
	}

	// Test GetUnencrypted sanitized secrets
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

	if _, err := configManager.DecryptSecret("enc:v1:aes256gcm:invalid:format:extra:extra"); err == nil {
		t.Fatal("expected error on invalid encrypted format")
	}

	// Test checkScope with invalid secret key
	service := NewService(kernel)
	configManager = service.configManager
	controlPlaneHandler := service.controlPlaneHandler
	serviceAccountManager := kernel.ServiceAccountManager()
	invalidKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/auth/config", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid-sec-key")
	if serviceAccountManager.CheckScope(invalidKeyRequest, core.ScopeAuthConfigRead) {
		t.Fatal("expected CheckScope to fail on invalid secret key")
	}

	// Test checkScope forbidden on handleGetConfig
	forbiddenGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetConfig(forbiddenGetResponseRecorder, invalidKeyRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on handleGetConfig, got: %d", forbiddenGetResponseRecorder.Code)
	}

	// Test checkScope forbidden on handleUpdateConfig
	forbiddenPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(forbiddenPutResponseRecorder, invalidKeyRequest)
	if forbiddenPutResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on handleUpdateConfig, got: %d", forbiddenPutResponseRecorder.Code)
	}

	// Test handleUpdateConfig success and event bus publish

	validPutBody := `{"password":{"enabled":true,"min_length":10},"smtp":{"password":"new-smtp-password"}}`
	validPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(validPutBody))
	validPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(validPutResponseRecorder, validPutRequest)
	if validPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig, got: %d (body: %s)", validPutResponseRecorder.Code, validPutResponseRecorder.Body.String())
	}
	if configManager.Get().Password.MinLength != 10 {
		t.Fatalf("expected updated min length 10, got: %d", configManager.Get().Password.MinLength)
	}

	// Test handleUpdateConfig invalid JSON body -> 400
	invalidJSONPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(`{invalid-json`))
	invalidJSONPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(invalidJSONPutResponseRecorder, invalidJSONPutRequest)
	if invalidJSONPutResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid JSON in handleUpdateConfig, got: %d", invalidJSONPutResponseRecorder.Code)
	}

	// Test handleUpdateConfig body read error -> 400
	brokenReaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", brokenBodyReader{})
	brokenReaderResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(brokenReaderResponseRecorder, brokenReaderRequest)
	if brokenReaderResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on read error in handleUpdateConfig, got: %d", brokenReaderResponseRecorder.Code)
	}

	// Test handleUpdateConfig with OAuth provider secret update
	oauthPutBody := `{"oauth_providers":{"google":{"enabled":true,"client_id":"new-client-id","client_secret":"new-plaintext-secret"}}}`
	oauthPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(oauthPutBody))
	oauthPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(oauthPutResponseRecorder, oauthPutRequest)
	if oauthPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with OAuth, got: %d", oauthPutResponseRecorder.Code)
	}
	if configManager.Get().OAuthProviders["google"].ClientID != "new-client-id" {
		t.Fatalf("expected updated OAuth client ID, got: %+v", configManager.Get().OAuthProviders["google"])
	}

	// Test handleUpdateConfig preserving OAuth secret when empty
	preserveOAuthBody := `{"oauth_providers":{"google":{"enabled":true,"client_id":"new-client-id","client_secret":""}}}`
	preserveOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(preserveOAuthBody))
	preserveOAuthResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(preserveOAuthResponseRecorder, preserveOAuthRequest)
	if preserveOAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig preserving OAuth secret, got: %d", preserveOAuthResponseRecorder.Code)
	}

	// Test handleUpdateConfig with null oauth_providers preserving current
	nullOAuthBody := `{"oauth_providers":null}`
	nullOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(nullOAuthBody))
	nullOAuthResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(nullOAuthResponseRecorder, nullOAuthRequest)
	if nullOAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with null oauth_providers, got: %d", nullOAuthResponseRecorder.Code)
	}

	// Test handleUpdateConfig with null oidc.clients preserving current
	nullOIDCBody := `{"oidc":{"clients":null}}`
	nullOIDCRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(nullOIDCBody))
	nullOIDCResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(nullOIDCResponseRecorder, nullOIDCRequest)
	if nullOIDCResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with null oidc.clients, got: %d", nullOIDCResponseRecorder.Code)
	}

	// Test handleUpdateConfig with null oidc.resource_servers preserving current
	nullResourceServersBody := `{"oidc":{"resource_servers":null}}`
	nullResourceServersRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(nullResourceServersBody))
	nullResourceServersResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(nullResourceServersResponseRecorder, nullResourceServersRequest)
	if nullResourceServersResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with null oidc.resource_servers, got: %d", nullResourceServersResponseRecorder.Code)
	}

	// Test handleUpdateConfig with Email and SMS plaintext secrets
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
	emailSMSPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(emailSMSPutBody))
	emailSMSPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(emailSMSPutResponseRecorder, emailSMSPutRequest)
	if emailSMSPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with Email and SMS, got: %d", emailSMSPutResponseRecorder.Code)
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

	// Test GetUnencrypted sanitized configuration for Email and SMS
	unencryptedConfig = configManager.GetUnencrypted()
	if !unencryptedConfig.EmailDispatcher.SMTP.PasswordConfigured || !unencryptedConfig.EmailDispatcher.Webhook.SigningSecretConfigured {
		t.Fatalf("expected email secrets configured in GetUnencrypted, got: %+v", unencryptedConfig.EmailDispatcher)
	}
	if !unencryptedConfig.SMSDispatcher.Twilio.AuthTokenConfigured || !unencryptedConfig.SMSDispatcher.Webhook.SigningSecretConfigured {
		t.Fatalf("expected sms secrets configured in GetUnencrypted, got: %+v", unencryptedConfig.SMSDispatcher)
	}

	// Test handleUpdateConfig with empty secrets preserving existing secrets
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
	preserveRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(preserveBody))
	preserveResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(preserveResponseRecorder, preserveRequest)
	if preserveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig preserving secrets, got: %d", preserveResponseRecorder.Code)
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
	nullDriverRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(nullDriverBody))
	nullDriverResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(nullDriverResponseRecorder, nullDriverRequest)
	if nullDriverResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with null driver, got: %d", nullDriverResponseRecorder.Code)
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
	emptyDriverRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(emptyDriverBody))
	emptyDriverResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(emptyDriverResponseRecorder, emptyDriverRequest)
	if emptyDriverResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with empty string driver, got: %d", emptyDriverResponseRecorder.Code)
	}
	emptyDriverConfig := configManager.Get()
	if emptyDriverConfig.EmailDispatcher.Driver != nil {
		t.Fatalf("expected email driver to be normalized to nil from empty string, got: %v", *emptyDriverConfig.EmailDispatcher.Driver)
	}
	if emptyDriverConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected sms driver to be normalized to nil from empty string, got: %v", *emptyDriverConfig.SMSDispatcher.Driver)
	}

	// Test that email and sms cannot be set to null via handleUpdateConfig
	nullConfigBody := `{
		"email_dispatcher": null,
		"sms_dispatcher": null
	}`
	nullConfigRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(nullConfigBody))
	nullConfigResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(nullConfigResponseRecorder, nullConfigRequest)
	if nullConfigResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with null email/sms, got: %d", nullConfigResponseRecorder.Code)
	}
	afterNullConfig := configManager.Get()
	if afterNullConfig.EmailDispatcher.Templates.EmailVerification.Subject == "" {
		t.Fatal("expected email templates to be preserved when payload sends null email")
	}
	if afterNullConfig.SMSDispatcher.Templates.PhoneVerification.Text == "" || afterNullConfig.SMSDispatcher.Templates.PasswordReset.Text == "" {
		t.Fatal("expected sms templates to be preserved when payload sends null sms")
	}

	// Test handleUpdateConfig with plaintext secrets
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
	plainRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(plainSecretBody))
	plainResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(plainResponseRecorder, plainRequest)
	if plainResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with plaintext secrets, got: %d", plainResponseRecorder.Code)
	}
	updatedPlainSecretsConfig := configManager.Get()
	if !strings.HasPrefix(updatedPlainSecretsConfig.EmailDispatcher.SMTP.Password, "enc:v1:") {
		t.Fatalf("expected encrypted SMTP password, got: %s", updatedPlainSecretsConfig.EmailDispatcher.SMTP.Password)
	}

	// Test handleUpdateConfig rejecting email_otp.enabled without active SMTP -> 422
	unconfiguredOTPPutBody := `{"email_otp":{"enabled":true}}`
	unconfiguredOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(unconfiguredOTPPutBody))
	unconfiguredOTPPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(unconfiguredOTPPutResponseRecorder, unconfiguredOTPPutRequest)
	if unconfiguredOTPPutResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on OTP enable without active SMTP, got: %d (%s)", unconfiguredOTPPutResponseRecorder.Code, unconfiguredOTPPutResponseRecorder.Body.String())
	}
	if !strings.Contains(unconfiguredOTPPutResponseRecorder.Body.String(), "SMTP is not configured") {
		t.Fatalf("expected SMTP is not configured error, got: %s", unconfiguredOTPPutResponseRecorder.Body.String())
	}

	// Test handleUpdateConfig rejecting sms_otp.enabled without active SMS provider -> 422
	unconfiguredSMSOTPPutBody := `{"sms_otp":{"enabled":true}}`
	unconfiguredSMSOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(unconfiguredSMSOTPPutBody))
	unconfiguredSMSOTPPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(unconfiguredSMSOTPPutResponseRecorder, unconfiguredSMSOTPPutRequest)
	if unconfiguredSMSOTPPutResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on SMS OTP enable without active SMS provider, got: %d (%s)", unconfiguredSMSOTPPutResponseRecorder.Code, unconfiguredSMSOTPPutResponseRecorder.Body.String())
	}
	if !strings.Contains(unconfiguredSMSOTPPutResponseRecorder.Body.String(), "SMS provider is not configured") {
		t.Fatalf("expected SMS provider is not configured error, got: %s", unconfiguredSMSOTPPutResponseRecorder.Body.String())
	}

	// Test handleUpdateConfig enabling OTP with active SMTP -> 200 OK
	configuredOTPPutBody := `{
		"email_otp":{"enabled":true},
		"email_dispatcher":{
			"driver":"smtp",
			"smtp":{"host":"smtp.example.com","port":587,"username":"user","password":"password"}
		}
	}`
	configuredOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(configuredOTPPutBody))
	configuredOTPPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(configuredOTPPutResponseRecorder, configuredOTPPutRequest)
	if configuredOTPPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on OTP enable with active SMTP, got: %d (%s)", configuredOTPPutResponseRecorder.Code, configuredOTPPutResponseRecorder.Body.String())
	}

	// Test handleUpdateConfig enabling SMS OTP with active SMS provider -> 200 OK
	configuredSMSOTPPutBody := `{
		"sms_otp":{"enabled":true},
		"sms_dispatcher":{
			"driver":"twilio",
			"twilio":{"account_sid":"AC123","auth_token":"token123","from_number":"+15551234567"}
		}
	}`
	configuredSMSOTPPutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(configuredSMSOTPPutBody))
	configuredSMSOTPPutResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(configuredSMSOTPPutResponseRecorder, configuredSMSOTPPutRequest)
	if configuredSMSOTPPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on SMS OTP enable with active SMS provider, got: %d (%s)", configuredSMSOTPPutResponseRecorder.Code, configuredSMSOTPPutResponseRecorder.Body.String())
	}
}

func TestAuthConfigManagerOIDCAndSignInUIUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	configManager := service.configManager
	controlPlaneHandler := service.controlPlaneHandler

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

	// 3. Test handleUpdateConfig with OIDC clients and UI
	oidcPutBody := `{
		"oidc": {
			"enabled": true,
			"ui": {
				"custom_css": ".auth-card { border-radius: 12px; }",
				"logo_url": "https://example.com/assets/logo.svg",
				"privacy_policy_url": "https://example.com/privacy",
				"terms_of_service_url": "https://example.com/terms",
				"show_password": true
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

	putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(oidcPutBody))
	putResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(putResponseRecorder, putRequest)

	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from handleUpdateConfig with OIDC config, got: %d (%s)", putResponseRecorder.Code, putResponseRecorder.Body.String())
	}

	savedConfig := configManager.Get()
	if !savedConfig.OIDC.Enabled {
		t.Fatal("expected saved OIDC.Enabled to be true")
	}
	if savedConfig.OIDC.UI.CustomCSS != ".auth-card { border-radius: 12px; }" {
		t.Fatalf("expected saved custom_css, got: %s", savedConfig.OIDC.UI.CustomCSS)
	}
	if savedConfig.OIDC.UI.LogoURL != "https://example.com/assets/logo.svg" {
		t.Fatalf("expected saved logo_url, got: %s", savedConfig.OIDC.UI.LogoURL)
	}
	if savedConfig.OIDC.UI.PrivacyPolicyURL != "https://example.com/privacy" {
		t.Fatalf("expected saved privacy_policy_url, got: %s", savedConfig.OIDC.UI.PrivacyPolicyURL)
	}
	if savedConfig.OIDC.UI.TermsOfServiceURL != "https://example.com/terms" {
		t.Fatalf("expected saved terms_of_service_url, got: %s", savedConfig.OIDC.UI.TermsOfServiceURL)
	}
	if !savedConfig.OIDC.UI.ShowPassword {
		t.Fatal("expected saved show_password to be true")
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

	// 4. Test GetUnencrypted sanitized configuration
	unencryptedConfig := configManager.GetUnencrypted()
	if unencryptedConfig.OIDC.UI.CustomCSS != ".auth-card { border-radius: 12px; }" {
		t.Fatalf("expected unencrypted custom_css preserved, got: %s", unencryptedConfig.OIDC.UI.CustomCSS)
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

	// 5. Test handleUpdateConfig preserving secret when empty client_secret is provided
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
	preserveRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(preserveSecretBody))
	preserveResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(preserveResponseRecorder, preserveRequest)

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

func TestAuthConfigUIValidationRejectionsUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	configManager := NewConfigManager(kernel)

	// Ensure base config has everything disabled initially
	initialConfig := configManager.Get()
	initialConfig.Password.Enabled = false
	initialConfig.Passkeys.Enabled = false
	initialConfig.EmailOTP.Enabled = false
	initialConfig.SMSOTP.Enabled = false
	for k := range initialConfig.OAuthProviders {
		oauthProviderConfig := initialConfig.OAuthProviders[k]
		oauthProviderConfig.Enabled = false
		initialConfig.OAuthProviders[k] = oauthProviderConfig
	}
	configManager.Set(initialConfig)

	testCases := []struct {
		name        string
		putBody     string
		expectedMsg string
	}{
		{
			name:        "reject show_password when password disabled",
			putBody:     `{"password": {"enabled": false}, "oidc": {"ui": {"show_password": true}}}`,
			expectedMsg: "Cannot enable show_password in UI config when password authentication is disabled",
		},
		{
			name:        "reject show_sign_up when password disabled",
			putBody:     `{"password": {"enabled": false}, "oidc": {"ui": {"show_sign_up": true}}}`,
			expectedMsg: "Cannot enable show_sign_up in UI config when password authentication is disabled",
		},
		{
			name:        "reject show_passkeys when passkeys disabled",
			putBody:     `{"passkeys": {"enabled": false}, "oidc": {"ui": {"show_passkeys": true}}}`,
			expectedMsg: "Cannot enable show_passkeys in UI config when passkey authentication is disabled",
		},
		{
			name:        "reject show_email_otp when email OTP disabled",
			putBody:     `{"email_otp": {"enabled": false}, "oidc": {"ui": {"show_email_otp": true}}}`,
			expectedMsg: "Cannot enable show_email_otp in UI config when email OTP authentication is disabled",
		},
		{
			name:        "reject show_sms_otp when sms OTP disabled",
			putBody:     `{"sms_otp": {"enabled": false}, "oidc": {"ui": {"show_sms_otp": true}}}`,
			expectedMsg: "Cannot enable show_sms_otp in UI config when SMS OTP authentication is disabled",
		},
		{
			name:        "reject show_oauth when no OAuth providers enabled",
			putBody:     `{"oauth_providers": {}, "oidc": {"ui": {"show_oauth": true}}}`,
			expectedMsg: "Cannot enable show_oauth in UI config when no OAuth providers are enabled",
		},
	}

	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	for _, currentTestCase := range testCases {
		t.Run(currentTestCase.name, func(t *testing.T) {
			validationRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(currentTestCase.putBody))
			validationResponseRecorder := httptest.NewRecorder()
			controlPlaneHandler.handleUpdateConfig(validationResponseRecorder, validationRequest)

			if validationResponseRecorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request, got: %d (%s)", validationResponseRecorder.Code, validationResponseRecorder.Body.String())
			}
			if !strings.Contains(validationResponseRecorder.Body.String(), currentTestCase.expectedMsg) {
				t.Fatalf("expected error message %q, got: %s", currentTestCase.expectedMsg, validationResponseRecorder.Body.String())
			}
		})
	}
}

func TestAuthConfigUIAutoSynchronizationUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	configManager := service.configManager
	controlPlaneHandler := service.controlPlaneHandler

	// 1. Initial state: disable all methods
	disableAllBody := `{
		"password": {"enabled": false},
		"passkeys": {"enabled": false},
		"email_otp": {"enabled": false},
		"sms_otp": {"enabled": false},
		"oauth_providers": {
			"google": {"enabled": false},
			"github": {"enabled": false}
		}
	}`
	disableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(disableAllBody))
	disableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(disableResponseRecorder, disableRequest)
	if disableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", disableResponseRecorder.Code)
	}

	disabledUIConfig := configManager.Get()
	if disabledUIConfig.OIDC.UI.ShowPassword || disabledUIConfig.OIDC.UI.ShowSignUp || disabledUIConfig.OIDC.UI.ShowPasskeys || disabledUIConfig.OIDC.UI.ShowEmailOTP || disabledUIConfig.OIDC.UI.ShowSMSOTP || disabledUIConfig.OIDC.UI.ShowOAuth {
		t.Fatalf("expected all UI show flags to be false when methods disabled, got: %+v", disabledUIConfig.OIDC.UI)
	}

	// 2. Enable methods one by one without explicit UI flags -> auto-enables UI show flags
	enableAllBody := `{
		"password": {"enabled": true},
		"passkeys": {"enabled": true},
		"email_otp": {"enabled": true},
		"sms_otp": {"enabled": true},
		"email_dispatcher": {
			"driver": "webhook",
			"webhook": {"url": "http://localhost:9999/webhook"}
		},
		"sms_dispatcher": {
			"driver": "webhook",
			"webhook": {"url": "http://localhost:9999/webhook"}
		},
		"oauth_providers": {
			"google": {"enabled": true, "preset": "google"}
		}
	}`
	enableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(enableAllBody))
	enableResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(enableResponseRecorder, enableRequest)
	if enableResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", enableResponseRecorder.Code)
	}

	enabledUIConfig := configManager.Get()
	if !enabledUIConfig.OIDC.UI.ShowPassword {
		t.Fatal("expected show_password to auto-sync to true")
	}
	if !enabledUIConfig.OIDC.UI.ShowPasskeys {
		t.Fatal("expected show_passkeys to auto-sync to true")
	}
	if !enabledUIConfig.OIDC.UI.ShowEmailOTP {
		t.Fatal("expected show_email_otp to auto-sync to true")
	}
	if !enabledUIConfig.OIDC.UI.ShowSMSOTP {
		t.Fatal("expected show_sms_otp to auto-sync to true")
	}
	if !enabledUIConfig.OIDC.UI.ShowOAuth {
		t.Fatal("expected show_oauth to auto-sync to true")
	}

	// 3. Disabling a method auto-syncs its show flag to false
	disablePassAndOAuth := `{
		"password": {"enabled": false},
		"oauth_providers": {
			"google": {"enabled": false}
		}
	}`
	disableMethodRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/_/auth/config", strings.NewReader(disablePassAndOAuth))
	disableMethodResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(disableMethodResponseRecorder, disableMethodRequest)
	if disableMethodResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", disableMethodResponseRecorder.Code)
	}

	resyncConfig := configManager.Get()
	if resyncConfig.OIDC.UI.ShowPassword {
		t.Fatal("expected show_password to auto-sync to false")
	}
	if resyncConfig.OIDC.UI.ShowSignUp {
		t.Fatal("expected show_sign_up to auto-sync to false")
	}
	if resyncConfig.OIDC.UI.ShowOAuth {
		t.Fatal("expected show_oauth to auto-sync to false")
	}
}

func TestAuthConfigRelyingPartyNameFallbackUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	configManager := NewConfigManager(kernel)

	defer core.SetLoadedConfig(core.DefaultConfig())

	// Case 1: Custom project name
	customProjectConfig := core.DefaultConfig()
	customProjectConfig.Project.Name = "Custom Company"
	core.SetLoadedConfig(customProjectConfig)

	defaultConfig := DefaultConfig()
	if defaultConfig.Passkeys.RelyingPartyName != "Custom Company" {
		t.Fatalf("expected relying party name 'Custom Company', got: %s", defaultConfig.Passkeys.RelyingPartyName)
	}
	if defaultConfig.MFA.Issuer != "Custom Company" {
		t.Fatalf("expected MFA issuer 'Custom Company', got: %s", defaultConfig.MFA.Issuer)
	}

	// In Set(): when empty, fallback to project name
	var inputConfig Config
	inputConfig.Passkeys.RelyingPartyName = ""
	inputConfig.MFA.Issuer = ""
	configManager.Set(inputConfig)
	if configManager.Get().Passkeys.RelyingPartyName != "Custom Company" {
		t.Fatalf("expected Set() to fallback to 'Custom Company', got: %s", configManager.Get().Passkeys.RelyingPartyName)
	}
	if configManager.Get().MFA.Issuer != "Custom Company" {
		t.Fatalf("expected Set() MFA issuer fallback to 'Custom Company', got: %s", configManager.Get().MFA.Issuer)
	}

	// Case 2: Empty project name -> falls back to "Layr Auth"
	emptyProjectConfig := core.DefaultConfig()
	emptyProjectConfig.Project.Name = ""
	core.SetLoadedConfig(emptyProjectConfig)

	emptyDefaultConfig := DefaultConfig()
	if emptyDefaultConfig.Passkeys.RelyingPartyName != "Layr Auth" {
		t.Fatalf("expected fallback to 'Layr Auth', got: %s", emptyDefaultConfig.Passkeys.RelyingPartyName)
	}
	if emptyDefaultConfig.MFA.Issuer != "Layr Auth" {
		t.Fatalf("expected fallback to 'Layr Auth', got: %s", emptyDefaultConfig.MFA.Issuer)
	}

	inputConfig.Passkeys.RelyingPartyName = ""
	inputConfig.MFA.Issuer = ""
	configManager.Set(inputConfig)
	if configManager.Get().Passkeys.RelyingPartyName != "Layr Auth" {
		t.Fatalf("expected Set() to fallback to 'Layr Auth', got: %s", configManager.Get().Passkeys.RelyingPartyName)
	}
	if configManager.Get().MFA.Issuer != "Layr Auth" {
		t.Fatalf("expected Set() MFA issuer to fallback to 'Layr Auth', got: %s", configManager.Get().MFA.Issuer)
	}
}

func TestAuthConfigOIDCClientSignOutFieldsUnit(t *testing.T) {
	configManager := NewConfigManager(core.SetupTestKernelWithBrokenDB(t, Migrations))

	// 1. JSON unmarshal using standard sign-out keys
	signOutJSON := []byte(`{
		"client_id": "client-signout",
		"name": "Sign Out Client",
		"backchannel_sign_out_uri": "https://example.com/api/sign-out",
		"backchannel_sign_out_session_required": true,
		"frontchannel_sign_out_uri": "https://example.com/front-sign-out",
		"frontchannel_sign_out_session_required": true
	}`)
	var standardOIDCClientConfig OIDCClientConfig
	if unmarshalErr := json.Unmarshal(signOutJSON, &standardOIDCClientConfig); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal standard sign-out JSON: %v", unmarshalErr)
	}
	if standardOIDCClientConfig.BackChannelSignOutURI != "https://example.com/api/sign-out" {
		t.Fatalf("unexpected backchannel signout uri: %s", standardOIDCClientConfig.BackChannelSignOutURI)
	}
	if !standardOIDCClientConfig.BackChannelSignOutSessionRequired {
		t.Fatal("expected backchannel signout session required to be true")
	}
	if standardOIDCClientConfig.FrontChannelSignOutURI != "https://example.com/front-sign-out" {
		t.Fatalf("unexpected frontchannel signout uri: %s", standardOIDCClientConfig.FrontChannelSignOutURI)
	}
	if !standardOIDCClientConfig.FrontChannelSignOutSessionRequired {
		t.Fatal("expected frontchannel signout session required to be true")
	}

	// 2. JSON unmarshal using OIDC wire aliases (logout_uri)
	logoutAliasJSON := []byte(`{
		"client_id": "client-logout-alias",
		"name": "Logout Alias Client",
		"backchannel_logout_uri": "https://example.com/api/logout-alias",
		"backchannel_logout_session_required": true,
		"frontchannel_logout_uri": "https://example.com/front-logout-alias",
		"frontchannel_logout_session_required": true
	}`)
	var aliasOIDCClientConfig OIDCClientConfig
	if unmarshalAliasErr := json.Unmarshal(logoutAliasJSON, &aliasOIDCClientConfig); unmarshalAliasErr != nil {
		t.Fatalf("failed to unmarshal alias logout JSON: %v", unmarshalAliasErr)
	}
	if aliasOIDCClientConfig.BackChannelSignOutURI != "https://example.com/api/logout-alias" {
		t.Fatalf("unexpected aliased backchannel signout uri: %s", aliasOIDCClientConfig.BackChannelSignOutURI)
	}
	if !aliasOIDCClientConfig.BackChannelSignOutSessionRequired {
		t.Fatal("expected aliased backchannel session required to be true")
	}
	if aliasOIDCClientConfig.FrontChannelSignOutURI != "https://example.com/front-logout-alias" {
		t.Fatalf("unexpected aliased frontchannel signout uri: %s", aliasOIDCClientConfig.FrontChannelSignOutURI)
	}
	if !aliasOIDCClientConfig.FrontChannelSignOutSessionRequired {
		t.Fatal("expected aliased frontchannel session required to be true")
	}

	// 3. Invalid JSON error branch
	var invalidOIDCClientConfig OIDCClientConfig
	if badJSONErr := json.Unmarshal([]byte(`{"backchannel_logout_session_required": "not-a-boolean"}`), &invalidOIDCClientConfig); badJSONErr == nil {
		t.Fatal("expected error on invalid JSON")
	}

	// 4. Set and Get preservation, and GetUnencrypted
	initialConfig := DefaultConfig()
	initialConfig.OIDC.Clients = []OIDCClientConfig{standardOIDCClientConfig}
	configManager.Set(initialConfig)

	retrievedConfig := configManager.Get()
	if len(retrievedConfig.OIDC.Clients) != 1 {
		t.Fatalf("expected 1 client in retrieved config, got: %d", len(retrievedConfig.OIDC.Clients))
	}
	if retrievedConfig.OIDC.Clients[0].BackChannelSignOutURI != "https://example.com/api/sign-out" {
		t.Fatalf("unexpected backchannel uri after Get(): %s", retrievedConfig.OIDC.Clients[0].BackChannelSignOutURI)
	}

	unencryptedConfig := configManager.GetUnencrypted()
	if len(unencryptedConfig.OIDC.Clients) != 1 {
		t.Fatalf("expected 1 client in unencrypted config, got: %d", len(unencryptedConfig.OIDC.Clients))
	}
	if unencryptedConfig.OIDC.Clients[0].BackChannelSignOutURI != "https://example.com/api/sign-out" {
		t.Fatalf("unexpected backchannel uri in unencrypted config: %s", unencryptedConfig.OIDC.Clients[0].BackChannelSignOutURI)
	}
	if unencryptedConfig.OIDC.Clients[0].FrontChannelSignOutURI != "https://example.com/front-sign-out" {
		t.Fatalf("unexpected frontchannel uri in unencrypted config: %s", unencryptedConfig.OIDC.Clients[0].FrontChannelSignOutURI)
	}
}

func TestAuthConfigThreatUnit(t *testing.T) {
	configManager := NewConfigManager(core.SetupTestKernelWithBrokenDB(t, Migrations))

	// 1. Default config assertions
	defaultConfig := DefaultConfig()
	if defaultConfig.Threat.BotProtection.Enabled {
		t.Fatal("expected BotProtection to be disabled by default")
	}
	if defaultConfig.Threat.BotProtection.Provider != "turnstile" {
		t.Fatalf("expected turnstile provider by default, got: %s", defaultConfig.Threat.BotProtection.Provider)
	}
	if defaultConfig.Threat.BotProtection.Mode != "adaptive" {
		t.Fatalf("expected adaptive mode by default, got: %s", defaultConfig.Threat.BotProtection.Mode)
	}
	if defaultConfig.Threat.BotProtection.AdaptiveFailedAttempts != 5 {
		t.Fatalf("expected 5 adaptive failed attempts by default, got: %d", defaultConfig.Threat.BotProtection.AdaptiveFailedAttempts)
	}
	if defaultConfig.Password.BreachCheck.Enabled {
		t.Fatal("expected password breach check disabled by default")
	}
	if !defaultConfig.Password.BreachCheck.FailOpen {
		t.Fatal("expected password breach check fail-open to be true by default")
	}
	if defaultConfig.MFA.Policy != "always" {
		t.Fatalf("expected default MFA policy always, got: %s", defaultConfig.MFA.Policy)
	}
	if len(defaultConfig.MFA.RiskTriggers) != 3 {
		t.Fatalf("expected 3 default MFA risk triggers, got: %d", len(defaultConfig.MFA.RiskTriggers))
	}

	// 2. Set with empty mode and non-positive failed attempts falls back to defaults
	customConfig := defaultConfig
	customConfig.Threat.BotProtection.Mode = ""
	customConfig.Threat.BotProtection.AdaptiveFailedAttempts = 0
	configManager.Set(customConfig)

	activeConfig := configManager.Get()
	if activeConfig.Threat.BotProtection.Mode != "adaptive" {
		t.Fatalf("expected fallback to adaptive mode, got: %s", activeConfig.Threat.BotProtection.Mode)
	}
	if activeConfig.Threat.BotProtection.AdaptiveFailedAttempts != 5 {
		t.Fatalf("expected fallback to 5 failed attempts, got: %d", activeConfig.Threat.BotProtection.AdaptiveFailedAttempts)
	}

	// 3. GetUnencrypted secret masking
	emptyUnencryptedConfig := configManager.GetUnencrypted()
	if emptyUnencryptedConfig.Threat.BotProtection.SecretKeyConfigured {
		t.Fatal("expected SecretKeyConfigured to be false when secret is empty")
	}
	if emptyUnencryptedConfig.Threat.BotProtection.SecretKey != "" {
		t.Fatalf("expected empty secret key in unencrypted config, got: %s", emptyUnencryptedConfig.Threat.BotProtection.SecretKey)
	}

	customConfig.Threat.BotProtection.SecretKey = "enc:v1:test-secret"
	configManager.Set(customConfig)

	setUnencryptedConfig := configManager.GetUnencrypted()
	if !setUnencryptedConfig.Threat.BotProtection.SecretKeyConfigured {
		t.Fatal("expected SecretKeyConfigured to be true when secret is set")
	}
	if setUnencryptedConfig.Threat.BotProtection.SecretKey != "" {
		t.Fatalf("expected masked secret key in unencrypted config, got: %s", setUnencryptedConfig.Threat.BotProtection.SecretKey)
	}
}

func TestAuthConfigThreatPutConfigUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	configManager := service.configManager
	controlPlaneHandler := service.controlPlaneHandler
	ctx := context.Background()

	// 1. Unsupported provider -> 400
	unsupportedProviderBody := `{"threat":{"bot_protection":{"enabled":true,"provider":"unknown_captcha","secret_key":"secret123"}}}`
	unsupportedRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader(unsupportedProviderBody))
	unsupportedResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(unsupportedResponseRecorder, unsupportedRequest)
	if unsupportedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unsupported provider, got: %d", unsupportedResponseRecorder.Code)
	}

	// 2. Enabled but missing secret key -> 400
	missingSecretBody := `{"threat":{"bot_protection":{"enabled":true,"provider":"turnstile","secret_key":""}}}`
	missingSecretRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader(missingSecretBody))
	missingSecretResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(missingSecretResponseRecorder, missingSecretRequest)
	if missingSecretResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing secret key, got: %d", missingSecretResponseRecorder.Code)
	}

	// 3. Invalid mode -> 400
	invalidModeBody := `{"threat":{"bot_protection":{"enabled":true,"provider":"turnstile","secret_key":"secret123","mode":"invalid_mode"}}}`
	invalidModeRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader(invalidModeBody))
	invalidModeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(invalidModeResponseRecorder, invalidModeRequest)
	if invalidModeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid mode, got: %d", invalidModeResponseRecorder.Code)
	}

	// 4. Valid update -> 200, secret is envelope encrypted
	validBody := `{"threat":{"bot_protection":{"enabled":true,"provider":"turnstile","secret_key":"my-turnstile-secret","mode":"adaptive","adaptive_failed_attempts":7}}}`
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader(validBody))
	validResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(validResponseRecorder, validRequest)
	if validResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid threat config update, got: %d (%s)", validResponseRecorder.Code, validResponseRecorder.Body.String())
	}
	savedConfig := configManager.Get()
	if !strings.HasPrefix(savedConfig.Threat.BotProtection.SecretKey, "enc:v1:") {
		t.Fatalf("expected encrypted secret key, got: %s", savedConfig.Threat.BotProtection.SecretKey)
	}
	if savedConfig.Threat.BotProtection.AdaptiveFailedAttempts != 7 {
		t.Fatalf("expected 7 adaptive failed attempts, got: %d", savedConfig.Threat.BotProtection.AdaptiveFailedAttempts)
	}

	// 5. Preserving existing secret when empty secret key sent
	preserveBody := `{"threat":{"bot_protection":{"enabled":true,"provider":"turnstile","secret_key":"","mode":"always"}}}`
	preserveRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", strings.NewReader(preserveBody))
	preserveResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(preserveResponseRecorder, preserveRequest)
	if preserveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on preserving threat secret, got: %d", preserveResponseRecorder.Code)
	}
	preservedConfig := configManager.Get()
	if preservedConfig.Threat.BotProtection.SecretKey != savedConfig.Threat.BotProtection.SecretKey {
		t.Fatalf("expected preserved encrypted secret, got: %s", preservedConfig.Threat.BotProtection.SecretKey)
	}
	if preservedConfig.Threat.BotProtection.Mode != "always" {
		t.Fatalf("expected mode updated to always, got: %s", preservedConfig.Threat.BotProtection.Mode)
	}
}
