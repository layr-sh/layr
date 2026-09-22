package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func smsDispatcherStringPointer(value string) *string {
	return &value
}

func TestAuthSMSDispatcherIsConfiguredUnit(t *testing.T) {
	// 1. Nil config provider
	var nilProviderSMSDispatcher *SMSDispatcher
	if nilProviderSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when dispatcher is nil")
	}

	// 2. Returns nil config
	nilSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return nil })
	if nilSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when config is nil")
	}

	// 3. Driver is nil
	nilDriverSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{Driver: nil}
	})
	if nilDriverSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when Driver is nil")
	}

	// 4. Incomplete Twilio: missing AccountSID
	twilioMissingAccountSIDSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("twilio"),
			Twilio: SMSDispatcherTwilioConfig{AccountSID: "", FromNumber: "+1555000", AuthToken: "token"},
		}
	})
	if twilioMissingAccountSIDSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when AccountSID is empty")
	}

	// Incomplete Twilio: missing FromNumber
	twilioMissingFromNumberSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("twilio"),
			Twilio: SMSDispatcherTwilioConfig{AccountSID: "AC123", FromNumber: "", AuthToken: "token"},
		}
	})
	if twilioMissingFromNumberSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when FromNumber is empty")
	}

	// Incomplete Twilio: missing AuthToken and not AuthTokenConfigured
	twilioMissingAuthTokenSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("twilio"),
			Twilio: SMSDispatcherTwilioConfig{AccountSID: "AC123", FromNumber: "+1555000", AuthToken: ""},
		}
	})
	if twilioMissingAuthTokenSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when AuthToken is empty and not configured")
	}

	// Complete Twilio with AuthToken
	twilioSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("twilio"),
			Twilio: SMSDispatcherTwilioConfig{AccountSID: "AC123", FromNumber: "+1555000", AuthToken: "token"},
		}
	})
	if !twilioSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete Twilio config with AuthToken")
	}

	// Complete Twilio with AuthTokenConfigured
	twilioAuthTokenConfiguredSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("twilio"),
			Twilio: SMSDispatcherTwilioConfig{AccountSID: "AC123", FromNumber: "+1555000", AuthTokenConfigured: true},
		}
	})
	if !twilioAuthTokenConfiguredSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete Twilio config with AuthTokenConfigured")
	}

	// Incomplete Webhook (empty URL)
	emptyWebhookSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  smsDispatcherStringPointer("webhook"),
			Webhook: SMSDispatcherWebhookConfig{URL: ""},
		}
	})
	if emptyWebhookSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when Webhook URL is empty")
	}

	// Complete Webhook
	webhookSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  smsDispatcherStringPointer("webhook"),
			Webhook: SMSDispatcherWebhookConfig{URL: "https://example.com/sms-webhook"},
		}
	})
	if !webhookSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete Webhook config")
	}

	// Unknown driver
	unknownSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver: smsDispatcherStringPointer("unknown"),
		}
	})
	if unknownSMSDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false for unknown driver")
	}
}

func TestAuthSMSSendUnconfiguredUnit(t *testing.T) {
	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return nil })
	ctx := context.Background()

	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+15551234567", Text: "Hello"}); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
	if err := smsDispatcher.SendSignInOTP(ctx, "+15551234567", "123456", ""); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
	if err := smsDispatcher.SendPhoneVerification(ctx, "+15551234567", "123456", ""); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
	if err := smsDispatcher.SendPasswordReset(ctx, "+15551234567", "123456", ""); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthSMSValidationErrorsUnit(t *testing.T) {
	driverTwilio := "twilio"
	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: &driverTwilio,
		Twilio: SMSDispatcherTwilioConfig{
			AccountSID: "AC123",
			FromNumber: "+1555000",
			AuthToken:  "token",
		},
	}
	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return smsDispatcherConfig })
	ctx := context.Background()

	// Empty recipient
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "", Text: "Body"}); !errors.Is(err, ErrSMSDispatcherInvalidRecipient) {
		t.Fatalf("expected ErrSMSDispatcherInvalidRecipient, got: %v", err)
	}
	// Empty body
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+15551234567", Text: "   "}); !errors.Is(err, ErrSMSDispatcherEmptyBody) {
		t.Fatalf("expected ErrSMSDispatcherEmptyBody, got: %v", err)
	}
	// Unknown driver
	driverUnknown := "unknown_driver"
	smsDispatcherConfig.Driver = &driverUnknown
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+15551234567", Text: "Body"}); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthSMSTemplateResolutionConfigUnit(t *testing.T) {
	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: smsDispatcherStringPointer("webhook"),
		Webhook: SMSDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
		Templates: SMSDispatcherTemplatesConfig{
			PasswordReset: SMSDispatcherTemplateConfig{
				Text: "Reset password for {{.Recipient}} with code {{.Code}} on {{.AppName}}",
			},
			SignInOTP: SMSDispatcherTemplateConfig{
				Text: "Sign in code for {{.Recipient}} is {{.Code}} on {{.AppName}}",
			},
			PhoneVerification: SMSDispatcherTemplateConfig{
				Text: "Verify {{.To}} with code {{.Code}}",
			},
		},
	}

	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return smsDispatcherConfig })
	ctx := context.Background()

	// 1. Password reset custom template
	text := smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "333444", "user-0", smsDispatcherConfig)
	if text != "Reset password for +15551234567 with code 333444 on layr-app" {
		t.Fatalf("unexpected custom password reset text: %s", text)
	}

	// 2. Sign in OTP custom template
	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "888999", "user-1", smsDispatcherConfig)
	if text != "Sign in code for +15551234567 is 888999 on layr-app" {
		t.Fatalf("unexpected custom sign in OTP text: %s", text)
	}

	// Test config.Get().Project.Name dynamic resolution
	testConfig := &core.Config{
		Project: core.ProjectConfig{Name: "CustomSMSApp"},
	}
	core.SetLoadedConfig(testConfig)
	defer core.SetLoadedConfig(nil)
	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "888999", "user-1", smsDispatcherConfig)
	if text != "Sign in code for +15551234567 is 888999 on CustomSMSApp" {
		t.Fatalf("expected custom AppName in template, got: %s", text)
	}
	core.SetLoadedConfig(nil)

	// Test config.Get().Project.Name fallback to "Layr" when empty
	testEmptyConfig := &core.Config{
		Project: core.ProjectConfig{Name: ""},
	}
	core.SetLoadedConfig(testEmptyConfig)
	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "888999", "user-1", smsDispatcherConfig)
	if text != "Sign in code for +15551234567 is 888999 on Layr" {
		t.Fatalf("expected Layr fallback in template, got: %s", text)
	}
	text = smsDispatcher.resolvePhoneVerificationTemplate(ctx, "+15551234567", "111222", "user-2", smsDispatcherConfig)
	if text != "Verify +15551234567 with code 111222" {
		t.Fatalf("unexpected custom phone verification text: %s", text)
	}
	text = smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "888999", "user-1", smsDispatcherConfig)
	if text != "Reset password for +15551234567 with code 888999 on Layr" {
		t.Fatalf("expected Layr fallback in password reset template, got: %s", text)
	}
	core.SetLoadedConfig(nil)

	// 3. Phone verification custom template
	text = smsDispatcher.resolvePhoneVerificationTemplate(ctx, "+15551234567", "111222", "user-2", smsDispatcherConfig)
	if text != "Verify +15551234567 with code 111222" {
		t.Fatalf("unexpected custom phone verification text: %s", text)
	}

	// 4. Defaults when templates are empty
	emptySMSDispatcherConfig := &SMSDispatcherConfig{}
	text = smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "888999", "", emptySMSDispatcherConfig)
	if !strings.Contains(text, "888999") || !strings.Contains(text, "password reset code") {
		t.Fatalf("unexpected default password reset text: %s", text)
	}

	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "888999", "", emptySMSDispatcherConfig)
	if !strings.Contains(text, "888999") || !strings.Contains(text, "sign in verification code") {
		t.Fatalf("unexpected default sign in OTP text: %s", text)
	}

	text = smsDispatcher.resolvePhoneVerificationTemplate(ctx, "+15551234567", "888999", "", emptySMSDispatcherConfig)
	if !strings.Contains(text, "888999") || !strings.Contains(text, "phone verification code") {
		t.Fatalf("unexpected default phone verification text: %s", text)
	}
}

func TestAuthSMSTwilioSendUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	cryptoKeyManager := kernel.CryptoKeyManager()

	encryptedAuthToken, _ := cryptoKeyManager.EncryptField([]byte("secret-twilio-token"))

	var capturedBody string
	var capturedAuthUser string
	var capturedAuthPass string

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		capturedBody = string(body)
		user, pass, _ := request.BasicAuth()
		capturedAuthUser = user
		capturedAuthPass = pass
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: smsDispatcherStringPointer("twilio"),
		Twilio: SMSDispatcherTwilioConfig{
			AccountSID: "AC1234567890",
			AuthToken:  encryptedAuthToken,
			FromNumber: "+15550001111",
		},
	}

	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return smsDispatcherConfig })
	smsDispatcher.httpClient = server.Client()

	// Override twilio endpoint in transport for test
	customClient := &http.Client{
		Transport: roundTripperFunc(func(outboundRequest *http.Request) (*http.Response, error) {
			if strings.Contains(outboundRequest.URL.Host, "api.twilio.com") {
				outboundRequest.URL.Scheme = "http"
				outboundRequest.URL.Host = server.Listener.Addr().String()
			}
			return http.DefaultTransport.RoundTrip(outboundRequest)
		}),
	}
	smsDispatcher.httpClient = customClient

	ctx := context.Background()
	err := smsDispatcher.SendSignInOTP(ctx, "+15559876543", "456123", "user-id")
	if err != nil {
		t.Fatalf("expected successful twilio send, got: %v", err)
	}

	if capturedAuthUser != "AC1234567890" || capturedAuthPass != "secret-twilio-token" {
		t.Fatalf("unexpected basic auth credentials: %s, %s", capturedAuthUser, capturedAuthPass)
	}
	if !strings.Contains(capturedBody, "%2B15559876543") || !strings.Contains(capturedBody, "456123") {
		t.Fatalf("unexpected form body: %s", capturedBody)
	}

	// Test empty AccountSID error (makes SMS dispatcher unconfigured)
	smsDispatcherConfig.Twilio.AccountSID = ""
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured on empty AccountSID, got: %v", err)
	}

	// Test corrupted encrypted token
	smsDispatcherConfig.Twilio.AccountSID = "AC1234567890"
	smsDispatcherConfig.Twilio.AuthToken = "enc:v1:corrupted:token"
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); err == nil {
		t.Fatal("expected decryption error on corrupted twilio token")
	}

	// Test non-2xx status code
	errServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusBadRequest)
	}))
	defer errServer.Close()

	smsDispatcherConfig.Twilio.AuthToken = "plain-token"
	smsDispatcher.httpClient = &http.Client{
		Transport: roundTripperFunc(func(outboundRequest *http.Request) (*http.Response, error) {
			outboundRequest.URL.Scheme = "http"
			outboundRequest.URL.Host = errServer.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(outboundRequest)
		}),
	}
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); err == nil {
		t.Fatal("expected error on non-2xx twilio status code")
	}

	// Test request construction failure with invalid control character in AccountSID
	smsDispatcherConfig.Twilio.AccountSID = "AC\x7fINVALID"
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); err == nil {
		t.Fatal("expected error creating request with invalid AccountSID")
	}

	// Test twilio network failure during client.Do
	smsDispatcherConfig.Twilio.AccountSID = "AC1234567890"
	smsDispatcher.httpClient = &http.Client{
		Transport: roundTripperFunc(func(outboundRequest *http.Request) (*http.Response, error) {
			return nil, errors.New("simulated twilio network failure")
		}),
	}
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); err == nil {
		t.Fatal("expected error on twilio network failure")
	}

	// Test twilio with nil httpClient falling back to http.DefaultClient
	{
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		smsDispatcher.httpClient = nil
		if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "Hi"}); err == nil {
			t.Fatal("expected error with canceled context on default client")
		}
	}
}

func TestAuthSMSWebhookSendUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	cryptoKeyManager := kernel.CryptoKeyManager()

	var receivedBody string
	var receivedSignature string
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		receivedBody = string(body)
		receivedSignature = request.Header.Get("X-Layr-Signature")
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	secret := "webhook-signing-secret"
	encryptedSecret, _ := cryptoKeyManager.EncryptField([]byte(secret))

	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: smsDispatcherStringPointer("webhook"),
		Webhook: SMSDispatcherWebhookConfig{
			URL:            server.URL,
			SigningSecret:  encryptedSecret,
			TimeoutSeconds: 5,
		},
	}

	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return smsDispatcherConfig })
	ctx := context.Background()

	err := smsDispatcher.SendPhoneVerification(ctx, "+15551234567", "654321", "user-id-xyz")
	if err != nil {
		t.Fatalf("expected successful webhook send, got: %v", err)
	}

	err = smsDispatcher.SendPasswordReset(ctx, "+15551234567", "654321", "user-id-xyz")
	if err != nil {
		t.Fatalf("expected successful webhook send for password reset, got: %v", err)
	}

	if !strings.Contains(receivedBody, `"+15551234567"`) || !strings.Contains(receivedBody, `"654321"`) {
		t.Fatalf("unexpected webhook payload: %s", receivedBody)
	}
	if !strings.HasPrefix(receivedSignature, "t=") || !strings.Contains(receivedSignature, "v1=") {
		t.Fatalf("expected X-Layr-Signature header, got: %s", receivedSignature)
	}

	// Test missing URL error (makes SMS dispatcher unconfigured)
	smsDispatcherConfig.Webhook.URL = ""
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "t"}); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured on empty webhook URL, got: %v", err)
	}

	// Test non-2xx status code
	errServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()
	smsDispatcherConfig.Webhook.URL = errServer.URL
	smsDispatcherConfig.Webhook.SigningSecret = "" // test without signing secret
	smsDispatcherConfig.Webhook.TimeoutSeconds = 0 // test timeout <= 0 fallback
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "t"}); err == nil {
		t.Fatal("expected error on non-2xx webhook status")
	}

	// Test corrupted encrypted secret
	smsDispatcherConfig.Webhook.SigningSecret = "enc:v1:corrupted:secret"
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "t"}); err == nil {
		t.Fatal("expected decryption error on corrupted secret")
	}

	// Test invalid URL causing request construction failure
	smsDispatcherConfig.Webhook.SigningSecret = ""
	smsDispatcherConfig.Webhook.URL = "http://[::1]:namedport"
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "t"}); err == nil {
		t.Fatal("expected error constructing request with invalid URL")
	}

	// Test webhook network failure during client.Do
	smsDispatcherConfig.Webhook.URL = "http://127.0.0.1:9999"
	smsDispatcher.httpClient = &http.Client{
		Transport: roundTripperFunc(func(outboundRequest *http.Request) (*http.Response, error) {
			return nil, errors.New("simulated webhook network failure")
		}),
	}
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555000", Text: "t"}); err == nil {
		t.Fatal("expected error on webhook network failure")
	}
}

func TestAuthSMSSendDefaultDriverAndClientUnit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: nil, // nil driver means unconfigured
		Twilio: SMSDispatcherTwilioConfig{
			AccountSID: "AC_DEFAULT_TEST",
			AuthToken:  "test_token",
			FromNumber: "+1555000",
		},
		Webhook: SMSDispatcherWebhookConfig{
			URL: server.URL,
		},
	}

	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return smsDispatcherConfig })

	// Test nil driver returns ErrSMSDispatcherNotConfigured
	ctx := context.Background()
	if err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555123", Text: "Test"}); !errors.Is(err, ErrSMSDispatcherNotConfigured) {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured on nil driver, got: %v", err)
	}

	// Test webhook driver with default nil httpClient
	smsDispatcher.httpClient = nil
	smsDispatcherConfig.Driver = smsDispatcherStringPointer("webhook")
	err := smsDispatcher.Send(ctx, SMSDispatcherMessage{To: "+1555123", Text: "Test"})
	if err != nil {
		t.Fatalf("expected successful webhook send with default client: %v", err)
	}
}

type roundTripperFunc func(outboundRequest *http.Request) (*http.Response, error)

func (roundTripHandler roundTripperFunc) RoundTrip(outboundRequest *http.Request) (*http.Response, error) {
	return roundTripHandler(outboundRequest)
}

func TestAuthSMSDispatcherDefaultConfigUnit(t *testing.T) {
	defaultSMSDispatcherConfig := SMSDispatcherDefaultConfig()
	if defaultSMSDispatcherConfig.Driver != nil {
		t.Fatalf("expected nil driver in SMSDispatcherDefaultConfig, got: %v", *defaultSMSDispatcherConfig.Driver)
	}
	if defaultSMSDispatcherConfig.Templates.PasswordReset.Text == "" {
		t.Fatalf("expected non-empty PasswordReset template, got: %+v", defaultSMSDispatcherConfig.Templates.PasswordReset)
	}
	if defaultSMSDispatcherConfig.Templates.PhoneVerification.Text == "" {
		t.Fatalf("expected non-empty PhoneVerification template, got: %+v", defaultSMSDispatcherConfig.Templates.PhoneVerification)
	}
	if defaultSMSDispatcherConfig.Templates.SignInOTP.Text == "" {
		t.Fatalf("expected non-empty SignInOTP template, got: %+v", defaultSMSDispatcherConfig.Templates.SignInOTP)
	}

	smsDispatcherConfig := SMSDispatcherDefaultConfig()
	if smsDispatcherConfig.Templates.PhoneVerification.Text != defaultSMSDispatcherConfig.Templates.PhoneVerification.Text {
		t.Fatal("expected SMSDispatcherDefaultConfig alias to match SMSDispatcherDefaultConfig")
	}
}
