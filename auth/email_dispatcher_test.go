package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func stringPointer(value string) *string {
	return &value
}

func TestAuthEmailDispatcherIsConfiguredUnit(t *testing.T) {
	// 1. Nil config provider
	nilEmailDispatcher := NewEmailDispatcher(nil, nil, nil)
	if nilEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when configProvider is nil")
	}

	// 2. Returns nil config
	nilConfigEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return nil }, nil)
	if nilConfigEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when config is nil")
	}

	// 3. Driver is nil
	nilDriverEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{Driver: nil}
	}, nil)
	if nilDriverEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when Driver is nil")
	}

	// 4. Incomplete SMTP driver configurations
	smtpEmptyHostEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "", Port: 587, Username: "user", Password: "pwd"},
		}
	}, nil)
	if smtpEmptyHostEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when SMTP host is empty")
	}

	smtpZeroPortEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "smtp.example.com", Port: 0, Username: "user", Password: "pwd"},
		}
	}, nil)
	if smtpZeroPortEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when SMTP port is 0")
	}

	emtpEmptyUserEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "smtp.example.com", Port: 587, Username: "", Password: "pwd"},
		}
	}, nil)
	if emtpEmptyUserEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when SMTP username is empty")
	}

	smtpEmptyPasswordEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "smtp.example.com", Port: 587, Username: "user", Password: ""},
		}
	}, nil)
	if smtpEmptyPasswordEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when SMTP password is empty and not configured")
	}

	// 5. Complete SMTP with Password
	smtpEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "smtp.example.com", Port: 587, Username: "user", Password: "pwd"},
		}
	}, nil)
	if !smtpEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete SMTP config with Password")
	}

	// 6. Complete SMTP with PasswordConfigured
	smtpPasswordConfiguredEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("smtp"),
			SMTP:   EmailDispatcherSMTPConfig{Host: "smtp.example.com", Port: 587, Username: "user", PasswordConfigured: true},
		}
	}, nil)
	if !smtpPasswordConfiguredEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete SMTP config with PasswordConfigured")
	}

	// 7. Incomplete Webhook (empty URL)
	emptyWebhookEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  stringPointer("webhook"),
			Webhook: EmailDispatcherWebhookConfig{URL: ""},
		}
	}, nil)
	if emptyWebhookEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false when Webhook URL is empty")
	}

	// 8. Complete Webhook
	webhookEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  stringPointer("webhook"),
			Webhook: EmailDispatcherWebhookConfig{URL: "https://example.com/webhook"},
		}
	}, nil)
	if !webhookEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be true for complete Webhook config")
	}

	// 9. Unknown driver
	unknownDriverEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver: stringPointer("unknown"),
		}
	}, nil)
	if unknownDriverEmailDispatcher.IsConfigured() {
		t.Fatal("expected IsConfigured() to be false for unknown driver")
	}
}

func TestAuthEmailSendUnconfiguredUnit(t *testing.T) {
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return nil }, nil)
	ctx := context.Background()

	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "Hi", Text: "Hello"}); !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
	if err := emailDispatcher.SendPasswordReset(ctx, "user@example.com", "123456", ""); !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
	if err := emailDispatcher.SendSignInOTP(ctx, "user@example.com", "123456", ""); !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
	if err := emailDispatcher.SendEmailVerification(ctx, "user@example.com", "123456", ""); !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthEmailValidationErrorsUnit(t *testing.T) {
	driverSMTP := "smtp"
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: &driverSMTP,
		SMTP: EmailDispatcherSMTPConfig{
			Host:     "127.0.0.1",
			Port:     587,
			Username: "user",
			Password: "pwd",
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	ctx := context.Background()

	// Empty recipient
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "", Subject: "Sub", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherInvalidRecipient) {
		t.Fatalf("expected ErrEmailDispatcherInvalidRecipient, got: %v", err)
	}
	// Invalid recipient missing @
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "invaliduser", Subject: "Sub", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherInvalidRecipient) {
		t.Fatalf("expected ErrEmailDispatcherInvalidRecipient, got: %v", err)
	}
	// Invalid recipient starting with @
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "@example.com", Subject: "Sub", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherInvalidRecipient) {
		t.Fatalf("expected ErrEmailDispatcherInvalidRecipient, got: %v", err)
	}
	// Invalid recipient ending with @
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@", Subject: "Sub", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherInvalidRecipient) {
		t.Fatalf("expected ErrEmailDispatcherInvalidRecipient, got: %v", err)
	}
	// Empty subject
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "   ", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherEmptySubject) {
		t.Fatalf("expected ErrEmailDispatcherEmptySubject, got: %v", err)
	}
	// Empty body
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "Sub", Text: "  ", HTML: ""}); !errors.Is(err, ErrEmailDispatcherEmptyBody) {
		t.Fatalf("expected ErrEmailDispatcherEmptyBody, got: %v", err)
	}
	// Unknown driver causes IsConfigured to return false and Send returns ErrEmailDispatcherNotConfigured
	driverUnknown := "unknown_driver"
	emailDispatcherConfig.Driver = &driverUnknown
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "Sub", Text: "Body"}); !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured for unknown driver, got: %v", err)
	}
}

func TestAuthEmailTemplateResolutionConfigUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("webhook"),
		SenderEmail: "custom@example.com",
		SenderName:  "Custom Name",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
		Templates: EmailDispatcherTemplatesConfig{
			PasswordReset: EmailDispatcherTemplateConfig{
				Subject: "Reset for {{.To}} on {{.AppName}}",
				HTML:    "<p>Code: {{.Code}}</p>",
				Text:    "Code: {{.Code}}",
			},
			SignInOTP: EmailDispatcherTemplateConfig{
				Subject: "OTP for {{.Recipient}}",
				HTML:    "<p>Your code: {{.Code}}</p>",
			},
			EmailVerification: EmailDispatcherTemplateConfig{
				Subject: "Verify email",
				Text:    "Your verification code: {{.Code}}",
			},
			SuspiciousActivity: EmailDispatcherTemplateConfig{
				Subject: "Suspicious login on {{.AppName}} for {{.Recipient}}",
				HTML:    "<p>Alert IP: {{.IP}} Device: {{.Device}}</p>",
				Text:    "Alert IP: {{.IP}} Device: {{.Device}}",
			},
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	ctx := context.Background()

	// 1. Password Reset custom template
	resolvedSubject, html, text := emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "999888", "user-1", emailDispatcherConfig)
	if resolvedSubject != "Reset for alice@example.com on layr-app" || html != "<p>Code: 999888</p>" || text != "Code: 999888" {
		t.Fatalf("unexpected template resolution: subject=%s, html=%s, text=%s", resolvedSubject, html, text)
	}

	// Dynamic config verification
	testConfig := &core.Config{
		Project: core.ProjectConfig{Name: "CustomProject"},
	}
	core.SetLoadedConfig(testConfig)
	defer core.SetLoadedConfig(nil)

	resolvedSubject, _, _ = emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "999888", "user-1", emailDispatcherConfig)
	if resolvedSubject != "Reset for alice@example.com on CustomProject" {
		t.Fatalf("expected AppName to interpolate CustomProject, got: %s", resolvedSubject)
	}
	core.SetLoadedConfig(nil)

	// 2. Sign In OTP custom template
	resolvedSubject, html, text = emailDispatcher.resolveSignInOTPTemplate(ctx, "bob@example.com", "111222", "user-2", emailDispatcherConfig)
	if resolvedSubject != "OTP for bob@example.com" || html != "<p>Your code: 111222</p>" {
		t.Fatalf("unexpected template resolution: subject=%s, html=%s, text=%s", resolvedSubject, html, text)
	}

	// 3. Email verification custom template
	resolvedSubject, html, text = emailDispatcher.resolveEmailVerificationTemplate(ctx, "charlie@example.com", "333444", "user-3", emailDispatcherConfig)
	if resolvedSubject != "Verify email" || text != "Your verification code: 333444" {
		t.Fatalf("unexpected template resolution: subject=%s, html=%s, text=%s", resolvedSubject, html, text)
	}

	// 4. Suspicious activity custom template
	resolvedSubject, html, text = emailDispatcher.resolveSuspiciousActivityTemplate(ctx, "david@example.com", "user-4", "192.168.1.1", "Mozilla/5.0", emailDispatcherConfig)
	if resolvedSubject != "Suspicious login on layr-app for david@example.com" || !strings.Contains(html, "192.168.1.1") || !strings.Contains(text, "Mozilla/5.0") {
		t.Fatalf("unexpected suspicious activity template resolution: subject=%s, html=%s, text=%s", resolvedSubject, html, text)
	}

	// 5. Default fallback when template is empty
	emptyEmailDispatcherConfig := &EmailDispatcherConfig{}
	resolvedSubject, html, text = emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "999888", "", emptyEmailDispatcherConfig)
	if resolvedSubject != "Reset your password" || !strings.Contains(html, "999888") || !strings.Contains(text, "999888") {
		t.Fatalf("unexpected default password reset: %s, %s, %s", resolvedSubject, html, text)
	}

	resolvedSubject, html, text = emailDispatcher.resolveSignInOTPTemplate(ctx, "alice@example.com", "999888", "", emptyEmailDispatcherConfig)
	if resolvedSubject != "Your sign in code" || !strings.Contains(html, "999888") || !strings.Contains(text, "999888") {
		t.Fatalf("unexpected default sign in OTP: %s, %s, %s", resolvedSubject, html, text)
	}

	resolvedSubject, html, text = emailDispatcher.resolveEmailVerificationTemplate(ctx, "alice@example.com", "999888", "", emptyEmailDispatcherConfig)
	if resolvedSubject != "Verify your email address" || !strings.Contains(html, "999888") || !strings.Contains(text, "999888") {
		t.Fatalf("unexpected default email verification: %s, %s, %s", resolvedSubject, html, text)
	}

	resolvedSubject, html, text = emailDispatcher.resolveSuspiciousActivityTemplate(ctx, "alice@example.com", "", "192.168.1.1", "Mozilla/5.0", emptyEmailDispatcherConfig)
	if resolvedSubject != "New sign-in detected on your account" || !strings.Contains(html, "192.168.1.1") || !strings.Contains(text, "Mozilla/5.0") {
		t.Fatalf("unexpected default suspicious activity: %s, %s, %s", resolvedSubject, html, text)
	}
}

func TestAuthEmailFormatMIMEUnit(t *testing.T) {
	// Both text and html
	multiPartEmailDispatcherMessage := EmailDispatcherMessage{
		To:          "user@example.com",
		Subject:     "Test Subject",
		Text:        "Plain text body",
		HTML:        "<h1>HTML body</h1>",
		SenderEmail: "sender@example.com",
		SenderName:  "Sender Name",
	}

	multiPartMIME := formatMIME(multiPartEmailDispatcherMessage, "sender@example.com")
	formattedMIME := string(multiPartMIME)
	if !strings.Contains(formattedMIME, "From: \"Sender Name\" <sender@example.com>") {
		t.Fatalf("expected From header, got: %s", formattedMIME)
	}
	if !strings.Contains(formattedMIME, "To: user@example.com") {
		t.Fatalf("expected To header, got: %s", formattedMIME)
	}
	if !strings.Contains(formattedMIME, "Content-Type: multipart/alternative;") {
		t.Fatalf("expected multipart alternative, got: %s", formattedMIME)
	}
	if !strings.Contains(formattedMIME, "Plain text body") || !strings.Contains(formattedMIME, "<h1>HTML body</h1>") {
		t.Fatalf("expected both text and html in mime, got: %s", formattedMIME)
	}

	// Default sender name fallback
	textOnlyEmailDispatcherMessage := EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Just text",
	}
	textOnlyMIME := formatMIME(textOnlyEmailDispatcherMessage, "fallback@example.com")
	if !strings.Contains(string(textOnlyMIME), "From: \"Layr\" <fallback@example.com>") {
		t.Fatalf("expected default sender name, got: %s", string(textOnlyMIME))
	}

	// Just HTML
	htmlOnlyEmailDispatcherMessage := EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		HTML:    "<p>Only html</p>",
	}
	htmlOnlyMIME := formatMIME(htmlOnlyEmailDispatcherMessage, "fallback@example.com")
	if !strings.Contains(string(htmlOnlyMIME), "<p>Only html</p>") {
		t.Fatalf("expected html in mime, got: %s", string(htmlOnlyMIME))
	}
}

func TestAuthEmailWebhookDeliveryUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

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

	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("webhook"),
		Webhook: EmailDispatcherWebhookConfig{
			URL:            server.URL,
			SigningSecret:  encryptedSecret,
			TimeoutSeconds: 5,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)
	ctx := context.Background()

	err = emailDispatcher.Send(ctx, EmailDispatcherMessage{
		Kind:    EmailDispatcherMessageKindSignInOTP,
		To:      "bob@example.com",
		Code:    "654321",
		Subject: "Sign in",
		Text:    "Your code is 654321",
	})
	if err != nil {
		t.Fatalf("expected successful webhook send, got: %v", err)
	}

	if !strings.Contains(receivedBody, `"bob@example.com"`) || !strings.Contains(receivedBody, `"654321"`) {
		t.Fatalf("unexpected webhook payload: %s", receivedBody)
	}
	if !strings.HasPrefix(receivedSignature, "t=") || !strings.Contains(receivedSignature, "v1=") {
		t.Fatalf("expected X-Layr-Signature header, got: %s", receivedSignature)
	}

	// Test missing URL error
	emailDispatcherConfig.Webhook.URL = ""
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on empty webhook URL")
	}

	// Test non-2xx status code
	errServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()
	emailDispatcherConfig.Webhook.URL = errServer.URL
	emailDispatcherConfig.Webhook.SigningSecret = "" // test without signing secret
	emailDispatcherConfig.Webhook.TimeoutSeconds = 0 // test timeout <= 0 fallback
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on non-2xx webhook status")
	}

	// Test corrupted encrypted secret
	emailDispatcherConfig.Webhook.SigningSecret = "enc:v1:corrupted:secret"
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected decryption error on corrupted secret")
	}
}

func TestAuthEmailSMTPSendUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	encryptedPassword, _ := cryptoKeyManager.EncryptField([]byte("secret-smtp-password"))

	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("smtp"),
		SenderEmail: "noreply@layr.sh",
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               2525,
			Username:           "testuser",
			Password:           encryptedPassword,
			TLSMode:            "none",
			InsecureSkipVerify: true,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)

	// Mock SMTP Server
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on TCP: %v", err)
	}
	defer func() { _ = listener.Close() }()

	tcpAddress := listener.Addr().(*net.TCPAddr)
	emailDispatcherConfig.SMTP.Port = tcpAddress.Port

	go func() {
		serverConnection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = serverConnection.Close() }()
		_ = serverConnection.SetDeadline(time.Now().Add(3 * time.Second))

		_, _ = serverConnection.Write([]byte("220 smtp.layr.local ESMTP\r\n"))
		readBuffer := make([]byte, 1024)
		for {
			bytesRead, readErr := serverConnection.Read(readBuffer)
			if readErr != nil {
				return
			}
			command := string(readBuffer[:bytesRead])
			if strings.HasPrefix(command, "EHLO") {
				_, _ = serverConnection.Write([]byte("250-smtp.layr.local\r\n250 AUTH PLAIN\r\n"))
			} else if strings.HasPrefix(command, "AUTH PLAIN") {
				_, _ = serverConnection.Write([]byte("235 Authentication successful\r\n"))
			} else if strings.HasPrefix(command, "MAIL FROM:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
			} else if strings.HasPrefix(command, "RCPT TO:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
			} else if strings.HasPrefix(command, "DATA") {
				_, _ = serverConnection.Write([]byte("354 Start mail input\r\n"))
			} else if strings.Contains(command, "\r\n.\r\n") || command == ".\r\n" {
				_, _ = serverConnection.Write([]byte("250 OK id=12345\r\n"))
			} else if strings.HasPrefix(command, "QUIT") {
				_, _ = serverConnection.Write([]byte("221 Bye\r\n"))
				return
			}
		}
	}()

	ctx := context.Background()
	err = emailDispatcher.SendPasswordReset(ctx, "user@example.com", "123456", "user-uuid")
	if err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}

	// Test connection failure to invalid port
	emailDispatcherConfig.SMTP.Port = 1 // unassigned/blocked port
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected connection error")
	}

	// Test corrupted password decryption error
	emailDispatcherConfig.SMTP.Port = tcpAddress.Port
	emailDispatcherConfig.SMTP.Password = "enc:v1:corrupted:password"
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		var dialListenConfig net.ListenConfig
		mockServerListener, listenErr := dialListenConfig.Listen(ctx, "tcp", "127.0.0.1:0")
		if listenErr != nil {
			return nil, fmt.Errorf("mock listen failed: %w", listenErr)
		}
		go func() {
			mockConnection, acceptErr := mockServerListener.Accept()
			if acceptErr != nil {
				return
			}
			defer func() { _ = mockConnection.Close() }()
			_ = mockConnection.SetDeadline(time.Now().Add(3 * time.Second))
			_, _ = mockConnection.Write([]byte("220 smtp.layr.local ESMTP\r\n"))
			mockBuffer := make([]byte, 1024)
			mockBytesRead, _ := mockConnection.Read(mockBuffer)
			if strings.HasPrefix(string(mockBuffer[:mockBytesRead]), "EHLO") {
				_, _ = mockConnection.Write([]byte("250-smtp.layr.local\r\n250 AUTH PLAIN\r\n"))
			}
		}()
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockServerListener.Addr().String())
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected decryption error on corrupted smtp password")
	}
}

func TestAuthEmailTLSAutoResolutionAndStartTLSUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "localhost",
			Port:               465,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "auto",
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)

	// Inject mock dialTLS that verifies tlsMode is resolved to direct TLS on port 465
	dialTLSCalled := false
	emailDispatcher.dialTLS = func(ctx context.Context, network, address string, tlsConfig *tls.Config) (*tls.Conn, error) {
		dialTLSCalled = true
		return nil, errors.New("mock tls dial failure")
	}

	ctx := context.Background()
	_ = emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "Test", Text: "Body"})
	if !dialTLSCalled {
		t.Fatal("expected dialTLS to be called for port 465 auto tls mode")
	}

	// Port != 465 auto resolves to starttls
	emailDispatcherConfig.SMTP.Port = 587
	dialTCPCalled := false
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialTCPCalled = true
		return nil, errors.New("mock tcp dial failure")
	}
	_ = emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "user@example.com", Subject: "Test", Text: "Body"})
	if !dialTCPCalled {
		t.Fatal("expected dialTCP to be called for port 587 auto tls mode")
	}
}

func TestAuthEmailSendWithEmailDispatcherTemplateConfiguredSenderUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("webhook"),
		SenderEmail: "custom-sender@example.com",
		SenderName:  "Custom Security Team",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
	}

	var sentEmailDispatcherMessage EmailDispatcherMessage
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	// Use mock client to capture sent message
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	emailDispatcherConfig.Webhook.URL = server.URL

	ctx := context.Background()
	err := emailDispatcher.SendSignInOTP(ctx, "bob@example.com", "777888", "user-id-123")
	if err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}

	// Also test with nil driver returning unconfigured
	emailDispatcherConfig.Driver = nil
	err = emailDispatcher.SendEmailVerification(ctx, "bob@example.com", "777888", "user-id-123")
	if !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
	_ = sentEmailDispatcherMessage
}

func TestAuthEmailSMTPExtendedErrorsUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "localhost",
			Port:               25,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "none",
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	ctx := context.Background()

	// 1. Client creation failure (server returns bad greeting)
	mockAddress1, cleanup1 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("500 Service Unavailable\r\n"))
	})
	defer cleanup1()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress1)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on bad greeting")
	}

	// 2. STARTTLS handshake failure
	emailDispatcherConfig.SMTP.TLSMode = "starttls"
	mockAddress2, cleanup2 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250-smtp.layr.local\r\n250 STARTTLS\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "STARTTLS") {
				_, _ = serverConnection.Write([]byte("500 STARTTLS Error\r\n"))
			}
		}
	})
	defer cleanup2()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress2)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on starttls failure")
	}

	// 3. Auth failure
	emailDispatcherConfig.SMTP.TLSMode = "none"
	emailDispatcherConfig.SMTP.Username = "user"
	emailDispatcherConfig.SMTP.Password = "pass"
	mockAddress3, cleanup3 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250-smtp.layr.local\r\n250 AUTH PLAIN\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "AUTH PLAIN") {
				_, _ = serverConnection.Write([]byte("535 Authentication failed\r\n"))
			}
		}
	})
	defer cleanup3()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress3)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on auth failure")
	}

	// 4. Mail From failure
	emailDispatcherConfig.SMTP.Username = "user"
	emailDispatcherConfig.SMTP.Password = ""
	emailDispatcherConfig.SMTP.PasswordConfigured = true
	mockAddress4, cleanup4 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250 smtp.layr.local\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "MAIL FROM:") {
				_, _ = serverConnection.Write([]byte("550 Sender rejected\r\n"))
			}
		}
	})
	defer cleanup4()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress4)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on mail from failure")
	}

	// 5. Rcpt To failure
	mockAddress5, cleanup5 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250 smtp.layr.local\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "MAIL FROM:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
				bytesRead3, _ := serverConnection.Read(readBuffer)
				if strings.HasPrefix(string(readBuffer[:bytesRead3]), "RCPT TO:") {
					_, _ = serverConnection.Write([]byte("550 Recipient rejected\r\n"))
				}
			}
		}
	})
	defer cleanup5()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress5)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on rcpt to failure")
	}

	// 6. Data command failure
	mockAddress6, cleanup6 := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250 smtp.layr.local\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "MAIL FROM:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
				bytesRead3, _ := serverConnection.Read(readBuffer)
				if strings.HasPrefix(string(readBuffer[:bytesRead3]), "RCPT TO:") {
					_, _ = serverConnection.Write([]byte("250 OK\r\n"))
					bytesRead4, _ := serverConnection.Read(readBuffer)
					if strings.HasPrefix(string(readBuffer[:bytesRead4]), "DATA") {
						_, _ = serverConnection.Write([]byte("500 Data command rejected\r\n"))
					}
				}
			}
		}
	})
	defer cleanup6()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress6)
	}
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error on data command failure")
	}
}

func createMockSMTPListener(t *testing.T, conversationHandler func(serverConnection net.Conn)) (string, func()) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	go func() {
		serverConnection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = serverConnection.Close() }()
		_ = serverConnection.SetDeadline(time.Now().Add(3 * time.Second))
		conversationHandler(serverConnection)
	}()
	return listener.Addr().String(), func() { _ = listener.Close() }
}

func TestAuthEmailWebhookRequestConstructionErrorUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("webhook"),
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://[::1]:namedport", // invalid port format causes http.NewRequestWithContext to fail
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	ctx := context.Background()
	if err := emailDispatcher.Send(ctx, EmailDispatcherMessage{To: "a@b.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error constructing request with invalid URL")
	}
}

func TestAuthEmailSendWithTemplateDefaultSenderUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("webhook"),
		SenderEmail: "",
		SenderName:  "",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&receivedPayload)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	emailDispatcherConfig.Webhook.URL = server.URL

	ctx := context.Background()

	// 1. When appName is configured, sender_name resolves to project name
	testConfig := &core.Config{
		Project: core.ProjectConfig{Name: "Acme Project"},
	}
	core.SetLoadedConfig(testConfig)
	defer core.SetLoadedConfig(nil)

	err := emailDispatcher.SendSignInOTP(ctx, "bob@example.com", "777888", "user-id-123")
	if err != nil {
		t.Fatalf("expected successful send with appName sender: %v", err)
	}
	if receivedPayload["sender_name"] != "Acme Project" {
		t.Fatalf("expected sender_name 'Acme Project', got: %v", receivedPayload["sender_name"])
	}

	// 2. When appName is empty, sender_name resolves to default
	testEmptyConfig := &core.Config{
		Project: core.ProjectConfig{Name: ""},
	}
	core.SetLoadedConfig(testEmptyConfig)

	err = emailDispatcher.SendSignInOTP(ctx, "bob@example.com", "777888", "user-id-123")
	if err != nil {
		t.Fatalf("expected successful send with default Layr sender: %v", err)
	}
	if receivedPayload["sender_name"] != "Layr" {
		t.Fatalf("expected sender_name 'Layr', got: %v", receivedPayload["sender_name"])
	}
	core.SetLoadedConfig(nil)

	// 3. When config has SenderName configured, direct Send resolves to emailDispatcherConfig.SenderName
	emailDispatcherConfig.SenderName = "Configured Sender"
	err = emailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "bob@example.com",
		Subject: "Subject",
		Text:    "Body",
	})
	if err != nil {
		t.Fatalf("expected successful direct Send: %v", err)
	}
	if receivedPayload["sender_name"] != "Configured Sender" {
		t.Fatalf("expected sender_name 'Configured Sender', got: %v", receivedPayload["sender_name"])
	}
}

func TestAuthEmailSMTPSendDefaultFallbackSenderUnit(t *testing.T) {
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("smtp"),
		SenderEmail: "",
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "localhost",
			Port:               25,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "none",
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	mockAddress, cleanup := createMockSMTPListener(t, func(serverConnection net.Conn) {
		_, _ = serverConnection.Write([]byte("220 smtp.layr.local\r\n"))
		readBuffer := make([]byte, 1024)
		bytesRead, _ := serverConnection.Read(readBuffer)
		if strings.HasPrefix(string(readBuffer[:bytesRead]), "EHLO") {
			_, _ = serverConnection.Write([]byte("250 smtp.layr.local\r\n"))
			bytesRead2, _ := serverConnection.Read(readBuffer)
			if strings.HasPrefix(string(readBuffer[:bytesRead2]), "MAIL FROM:<no-reply@layr.sh>") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
				bytesRead3, _ := serverConnection.Read(readBuffer)
				if strings.HasPrefix(string(readBuffer[:bytesRead3]), "RCPT TO:") {
					_, _ = serverConnection.Write([]byte("250 OK\r\n"))
					bytesRead4, _ := serverConnection.Read(readBuffer)
					if strings.HasPrefix(string(readBuffer[:bytesRead4]), "DATA") {
						_, _ = serverConnection.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
						_, _ = serverConnection.Read(readBuffer)
						_, _ = serverConnection.Write([]byte("250 OK\r\n"))
					}
				}
			}
		}
	})
	defer cleanup()
	emailDispatcher.dialTCP = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", mockAddress)
	}

	ctx := context.Background()
	err := emailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:          "user@example.com",
		Subject:     "Subject",
		Text:        "Text body",
		SenderEmail: "",
	})
	if err != nil {
		t.Fatalf("expected successful send with fallback sender: %v", err)
	}
}

func TestAuthEmailWebhookMissingURLAndDefaultTimeoutUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Missing URL
	webhookMissingURLEmailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("webhook"),
		Webhook: EmailDispatcherWebhookConfig{
			URL: "",
		},
	}
	webhookMissingURLEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return webhookMissingURLEmailDispatcherConfig }, nil)
	err := webhookMissingURLEmailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if !errors.Is(err, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured for empty webhook URL, got: %v", err)
	}

	// 2. Default Timeout <= 0 and DefaultClient == nil
	deliveredChannel := make(chan bool, 1)
	testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		deliveredChannel <- true
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	webhookZeroTimeoutEmailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("webhook"),
		Webhook: EmailDispatcherWebhookConfig{
			URL:            testServer.URL,
			TimeoutSeconds: 0, // Triggers timeout <= 0 branch
		},
	}
	webhookZeroTimeoutEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return webhookZeroTimeoutEmailDispatcherConfig }, nil)
	webhookZeroTimeoutEmailDispatcher.httpClient = nil // Triggers client == nil branch

	err = webhookZeroTimeoutEmailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if err != nil {
		t.Fatalf("expected send success with default timeout and client, got: %v", err)
	}
	<-deliveredChannel
}

func TestAuthEmailDefaultTLSContextAndHostPortFallbacksUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Tests host == "" and port <= 0
	smtpEmptyHostEmailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "", // Incomplete host
			Port:               0,
			Username:           "user",
			PasswordConfigured: true,
		},
	}
	smtpEmptyHostEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return smtpEmptyHostEmailDispatcherConfig }, nil)
	if smtpEmptyHostEmailDispatcher.IsConfigured() {
		t.Fatal("expected empty host/port to be unconfigured")
	}

	// 2. Tests default dialTLSContext when dispatcher.dialTLS == nil
	mockTLSServer := httptest.NewTLSServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {}))
	tlsConfig := mockTLSServer.TLS
	mockTLSServer.Close()

	tlsListener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to listen on tls: %v", err)
	}
	defer func() { _ = tlsListener.Close() }()

	go func() {
		serverConnection, acceptErr := tlsListener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = serverConnection.Close() }()
		_, _ = serverConnection.Write([]byte("500 SMTP Service Unavailable\r\n"))
	}()

	tlsPort := tlsListener.Addr().(*net.TCPAddr).Port
	mockTLSEmailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               tlsPort,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "tls",
			InsecureSkipVerify: true,
		},
	}
	tlsEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return mockTLSEmailDispatcherConfig }, nil)
	// tlsEmailDispatcher.dialTLS is nil, so it will execute the default dialTLSContext function and hit line 322!
	tlsEmailDispatcherSendErr := tlsEmailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if tlsEmailDispatcherSendErr == nil {
		t.Fatal("expected error after TLS connection established to non-smtp server")
	}

	closedTLSEmailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               1,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "tls",
			InsecureSkipVerify: true,
		},
	}
	closedTLSEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return closedTLSEmailDispatcherConfig }, nil)
	closedTLSEmailDispatcherSendErr := closedTLSEmailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if closedTLSEmailDispatcherSendErr == nil {
		t.Fatal("expected error connecting to closed TLS port")
	}
}

func TestAuthEmailWebhookClientDoErrorUnit(t *testing.T) {
	ctx := context.Background()
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("webhook"),
		Webhook: EmailDispatcherWebhookConfig{
			URL:            "http://127.0.0.1:1", // Closed/privileged port triggers client.Do error
			TimeoutSeconds: 1,
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)
	err := emailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if err == nil || !strings.Contains(err.Error(), "webhook request failed") {
		t.Fatalf("expected webhook request failed error, got: %v", err)
	}
}

func TestAuthEmailSMTPDataWriteErrorUnit(t *testing.T) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on TCP: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		serverConnection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = serverConnection.Close() }()
		_ = serverConnection.SetDeadline(time.Now().Add(5 * time.Second))

		_, _ = serverConnection.Write([]byte("220 smtp.layr.local ESMTP Mock\r\n"))
		readBuffer := make([]byte, 1024)
		for {
			bytesRead, readErr := serverConnection.Read(readBuffer)
			if readErr != nil {
				return
			}
			command := string(readBuffer[:bytesRead])
			if strings.HasPrefix(command, "EHLO") || strings.HasPrefix(command, "HELO") {
				_, _ = serverConnection.Write([]byte("250 smtp.layr.local\r\n"))
			} else if strings.HasPrefix(command, "MAIL FROM:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
			} else if strings.HasPrefix(command, "RCPT TO:") {
				_, _ = serverConnection.Write([]byte("250 OK\r\n"))
			} else if strings.HasPrefix(command, "DATA") {
				_, _ = serverConnection.Write([]byte("354 Start mail input\r\n"))
				// Close connection immediately so client dataWriter.Write fails
				_ = serverConnection.Close()
				return
			}
		}
	}()

	tcpAddress := listener.Addr().(*net.TCPAddr)
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: stringPointer("smtp"),
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               tcpAddress.Port,
			Username:           "testuser",
			PasswordConfigured: true,
			TLSMode:            "none",
			InsecureSkipVerify: true,
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)

	ctx := context.Background()
	sendErr := emailDispatcher.Send(ctx, EmailDispatcherMessage{
		To:      "user@example.com",
		Subject: "Test",
		Text:    "Body",
	})
	if sendErr == nil || !strings.Contains(sendErr.Error(), "smtp data delivery failed") {
		t.Fatalf("expected data delivery error, got: %v", sendErr)
	}
}

func TestAuthDefaultEmailConfigUnit(t *testing.T) {
	defaultEmailDispatcherConfig := EmailDispatcherDefaultConfig()
	if defaultEmailDispatcherConfig.Driver != nil {
		t.Fatalf("expected nil driver in DefaultEmailConfig, got: %v", *defaultEmailDispatcherConfig.Driver)
	}
	if defaultEmailDispatcherConfig.SenderName != "" {
		t.Fatalf("expected empty SenderName in DefaultEmailConfig, got: %s", defaultEmailDispatcherConfig.SenderName)
	}
	if defaultEmailDispatcherConfig.Templates.EmailVerification.Subject == "" || defaultEmailDispatcherConfig.Templates.EmailVerification.HTML == "" || defaultEmailDispatcherConfig.Templates.EmailVerification.Text == "" {
		t.Fatalf("expected non-empty EmailVerification templates, got: %+v", defaultEmailDispatcherConfig.Templates.EmailVerification)
	}
	if defaultEmailDispatcherConfig.Templates.PasswordReset.Subject == "" || defaultEmailDispatcherConfig.Templates.PasswordReset.HTML == "" || defaultEmailDispatcherConfig.Templates.PasswordReset.Text == "" {
		t.Fatalf("expected non-empty PasswordReset templates, got: %+v", defaultEmailDispatcherConfig.Templates.PasswordReset)
	}
	if defaultEmailDispatcherConfig.Templates.SignInOTP.Subject == "" || defaultEmailDispatcherConfig.Templates.SignInOTP.HTML == "" || defaultEmailDispatcherConfig.Templates.SignInOTP.Text == "" {
		t.Fatalf("expected non-empty SignInOTP templates, got: %+v", defaultEmailDispatcherConfig.Templates.SignInOTP)
	}
	if defaultEmailDispatcherConfig.Templates.SuspiciousActivity.Subject == "" || defaultEmailDispatcherConfig.Templates.SuspiciousActivity.HTML == "" || defaultEmailDispatcherConfig.Templates.SuspiciousActivity.Text == "" {
		t.Fatalf("expected non-empty SuspiciousActivity templates, got: %+v", defaultEmailDispatcherConfig.Templates.SuspiciousActivity)
	}

	aliasEmailDispatcherConfig := EmailDispatcherDefaultConfig()
	if aliasEmailDispatcherConfig.Templates.EmailVerification.Subject != defaultEmailDispatcherConfig.Templates.EmailVerification.Subject {
		t.Fatal("expected EmailDispatcherDefaultConfig alias to match DefaultEmailConfig")
	}
}

func TestAuthEmailSuspiciousActivityUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Unconfigured dispatcher returns ErrEmailDispatcherNotConfigured
	unconfiguredEmailDispatcher := NewEmailDispatcher(nil, nil, nil)
	unconfErr := unconfiguredEmailDispatcher.SendSuspiciousActivity(ctx, "user@example.com", "usr_1", "127.0.0.1", "curl")
	if !errors.Is(unconfErr, ErrEmailDispatcherNotConfigured) {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", unconfErr)
	}

	// 2. Webhook delivery with default template and default app name
	var receivedPayload map[string]any
	webhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&receivedPayload)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("webhook"),
		SenderEmail: "",
		SenderName:  "Security Bot",
		Webhook: EmailDispatcherWebhookConfig{
			URL: webhookServer.URL,
		},
	}
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, nil)

	// App name is empty -> defaults to "layr-app"
	core.SetLoadedConfig(&core.Config{Project: core.ProjectConfig{Name: ""}})
	defer core.SetLoadedConfig(nil)

	err := emailDispatcher.SendSuspiciousActivity(ctx, "victim@example.com", "usr_123", "203.0.113.195", "Firefox on Linux")
	if err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}
	if receivedPayload["message_kind"] != "suspicious_activity" {
		t.Fatalf("unexpected message_kind: %v", receivedPayload["message_kind"])
	}
	if receivedPayload["to"] != "victim@example.com" {
		t.Fatalf("unexpected to: %v", receivedPayload["to"])
	}
	if receivedPayload["user_id"] != "usr_123" {
		t.Fatalf("unexpected user_id: %v", receivedPayload["user_id"])
	}
	if receivedPayload["sender_email"] != "no-reply@layr.sh" {
		t.Fatalf("expected default sender email no-reply@layr.sh, got: %v", receivedPayload["sender_email"])
	}
	if !strings.Contains(receivedPayload["text"].(string), "layr-app") {
		t.Fatalf("expected text to contain default app name layr-app, got: %v", receivedPayload["text"])
	}
	if !strings.Contains(receivedPayload["text"].(string), "203.0.113.195") || !strings.Contains(receivedPayload["text"].(string), "Firefox on Linux") {
		t.Fatalf("expected text body to contain IP and device info, got: %v", receivedPayload["text"])
	}

	// 3. Webhook delivery with custom app name and configured sender email
	core.SetLoadedConfig(&core.Config{Project: core.ProjectConfig{Name: "AcmeAuth"}})
	emailDispatcherConfig.SenderEmail = "alerts@example.com"

	err = emailDispatcher.SendSuspiciousActivity(ctx, "victim@example.com", "usr_123", "198.51.100.5", "Safari on iOS")
	if err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}
	if receivedPayload["sender_email"] != "alerts@example.com" {
		t.Fatalf("expected sender_email alerts@example.com, got: %v", receivedPayload["sender_email"])
	}
	if !strings.Contains(receivedPayload["text"].(string), "AcmeAuth") {
		t.Fatalf("expected text to contain AcmeAuth, got: %v", receivedPayload["text"])
	}

	// 4. Custom templates
	emailDispatcherConfig.Templates.SuspiciousActivity = EmailDispatcherTemplateConfig{
		Subject: "Alert: {{.AppName}} login from {{.IP}}",
		HTML:    "<p>Device: {{.Device}} for {{.Recipient}} / {{.To}}</p>",
		Text:    "Device: {{.Device}} for {{.Recipient}} / {{.To}}",
	}
	err = emailDispatcher.SendSuspiciousActivity(ctx, "custom@example.com", "usr_456", "1.2.3.4", "CustomDevice")
	if err != nil {
		t.Fatalf("expected successful send with custom templates: %v", err)
	}
	if receivedPayload["subject"] != "Alert: AcmeAuth login from 1.2.3.4" {
		t.Fatalf("unexpected custom subject: %v", receivedPayload["subject"])
	}
	if receivedPayload["text"] != "Device: CustomDevice for custom@example.com / custom@example.com" {
		t.Fatalf("unexpected custom text: %v", receivedPayload["text"])
	}

	// 5. Incomplete custom template (subject provided, but empty html and text) falls back to default
	emailDispatcherConfig.Templates.SuspiciousActivity = EmailDispatcherTemplateConfig{
		Subject: "Custom Subject Only",
		HTML:    "",
		Text:    "",
	}
	err = emailDispatcher.SendSuspiciousActivity(ctx, "custom@example.com", "usr_456", "1.2.3.4", "CustomDevice")
	if err != nil {
		t.Fatalf("expected successful send with fallback template: %v", err)
	}
	if !strings.Contains(receivedPayload["text"].(string), "A new sign-in was detected") {
		t.Fatalf("expected default text template fallback, got: %v", receivedPayload["text"])
	}
}
