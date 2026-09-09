package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthEmailUnconfiguredPasswordResetE2E(t *testing.T) {
	ctx := context.Background()
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return nil }, nil)

	err := emailDispatcher.SendPasswordReset(ctx, "recipient@example.com", "123456", "user-uuid")
	if err != ErrEmailDispatcherNotConfigured {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthEmailUnconfiguredOTPSendE2E(t *testing.T) {
	ctx := context.Background()
	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{Driver: nil}
	}, nil)

	err := emailDispatcher.SendSignInOTP(ctx, "recipient@example.com", "123456", "user-uuid")
	if err != ErrEmailDispatcherNotConfigured {
		t.Fatalf("expected ErrEmailDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthEmailPasswordResetDispatchE2E(t *testing.T) {
	ctx := context.Background()
	receivedMail := make(chan string, 1)

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind mock smtp listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = connection.Close() }()

		reader := bufio.NewReader(connection)
		_, _ = connection.Write([]byte("220 smtp.layr.internal Service Ready\r\n"))

		var messageData strings.Builder
		inDataSection := false

		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				break
			}
			trimmedLine := strings.TrimRight(line, "\r\n")

			if inDataSection {
				if trimmedLine == "." {
					inDataSection = false
					_, _ = connection.Write([]byte("250 2.0.0 OK message queued\r\n"))
					receivedMail <- messageData.String()
				} else {
					messageData.WriteString(line)
				}
				continue
			}

			switch {
			case strings.HasPrefix(trimmedLine, "EHLO") || strings.HasPrefix(trimmedLine, "HELO"):
				_, _ = connection.Write([]byte("250-smtp.layr.internal\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(trimmedLine, "AUTH PLAIN"):
				_, _ = connection.Write([]byte("235 2.7.0 Authentication successful\r\n"))
			case strings.HasPrefix(trimmedLine, "MAIL FROM:"):
				_, _ = connection.Write([]byte("250 2.1.0 Sender OK\r\n"))
			case strings.HasPrefix(trimmedLine, "RCPT TO:"):
				_, _ = connection.Write([]byte("250 2.1.5 Recipient OK\r\n"))
			case trimmedLine == "DATA":
				inDataSection = true
				_, _ = connection.Write([]byte("354 Start mail input; end with <CRLF>.<CRLF>\r\n"))
			case trimmedLine == "QUIT":
				_, _ = connection.Write([]byte("221 2.0.0 Bye\r\n"))
				return
			default:
				_, _ = connection.Write([]byte("500 Unrecognized command\r\n"))
			}
		}
	}()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	encryptedPassword, err := cryptoKeyManager.EncryptField([]byte("secret-password"))
	if err != nil {
		t.Fatalf("failed to encrypt password: %v", err)
	}

	smtpPort := listener.Addr().(*net.TCPAddr).Port
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("smtp"),
		SenderEmail: "system@layr.sh",
		SenderName:  "Layr Security",
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               smtpPort,
			Username:           "layruser",
			Password:           encryptedPassword,
			TLSMode:            "none",
			InsecureSkipVerify: true,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)
	sendErr := emailDispatcher.SendPasswordReset(ctx, "user@example.com", "987654", "user-uuid")
	if sendErr != nil {
		t.Fatalf("failed to send password reset: %v", sendErr)
	}

	receivedContent := <-receivedMail
	if !strings.Contains(receivedContent, "Subject:") {
		t.Fatalf("expected subject in mail, got: %s", receivedContent)
	}
	if !strings.Contains(receivedContent, "987654") {
		t.Fatalf("expected code in mail, got: %s", receivedContent)
	}
}

func TestAuthEmailSignInOTPDispatchE2E(t *testing.T) {
	ctx := context.Background()
	receivedMail := make(chan string, 1)

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind mock smtp listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = connection.Close() }()

		reader := bufio.NewReader(connection)
		_, _ = connection.Write([]byte("220 smtp.layr.internal Service Ready\r\n"))

		var messageData strings.Builder
		inDataSection := false

		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				break
			}
			trimmedLine := strings.TrimRight(line, "\r\n")

			if inDataSection {
				if trimmedLine == "." {
					inDataSection = false
					_, _ = connection.Write([]byte("250 2.0.0 OK message queued\r\n"))
					receivedMail <- messageData.String()
				} else {
					messageData.WriteString(line)
				}
				continue
			}

			switch {
			case strings.HasPrefix(trimmedLine, "EHLO") || strings.HasPrefix(trimmedLine, "HELO"):
				_, _ = connection.Write([]byte("250-smtp.layr.internal\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(trimmedLine, "AUTH PLAIN"):
				_, _ = connection.Write([]byte("235 2.7.0 Authentication successful\r\n"))
			case strings.HasPrefix(trimmedLine, "MAIL FROM:"):
				_, _ = connection.Write([]byte("250 2.1.0 Sender OK\r\n"))
			case strings.HasPrefix(trimmedLine, "RCPT TO:"):
				_, _ = connection.Write([]byte("250 2.1.5 Recipient OK\r\n"))
			case trimmedLine == "DATA":
				inDataSection = true
				_, _ = connection.Write([]byte("354 Start mail input; end with <CRLF>.<CRLF>\r\n"))
			case trimmedLine == "QUIT":
				_, _ = connection.Write([]byte("221 2.0.0 Bye\r\n"))
				return
			default:
				_, _ = connection.Write([]byte("500 Unrecognized command\r\n"))
			}
		}
	}()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	encryptedPassword, err := cryptoKeyManager.EncryptField([]byte("secret-password"))
	if err != nil {
		t.Fatalf("failed to encrypt password: %v", err)
	}

	smtpPort := listener.Addr().(*net.TCPAddr).Port
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("smtp"),
		SenderEmail: "system@layr.sh",
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               smtpPort,
			Username:           "layruser",
			Password:           encryptedPassword,
			TLSMode:            "none",
			InsecureSkipVerify: true,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)
	sendErr := emailDispatcher.SendSignInOTP(ctx, "user@example.com", "555123", "user-uuid")
	if sendErr != nil {
		t.Fatalf("failed to send sign-in otp: %v", sendErr)
	}

	receivedContent := <-receivedMail
	if !strings.Contains(receivedContent, "Subject:") {
		t.Fatalf("expected subject in mail, got: %s", receivedContent)
	}
	if !strings.Contains(receivedContent, "555123") {
		t.Fatalf("expected code in mail, got: %s", receivedContent)
	}
}

func TestAuthEmailWebhookDispatchE2E(t *testing.T) {
	ctx := context.Background()
	payloadDelivered := make(chan map[string]any, 1)

	webhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		signatureHeader := request.Header.Get("X-Layr-Signature")
		if signatureHeader == "" || !strings.Contains(signatureHeader, "v1=") {
			http.Error(responseWriter, "missing or invalid signature", http.StatusBadRequest)
			return
		}

		bodyBytes, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			http.Error(responseWriter, "read error", http.StatusBadRequest)
			return
		}

		var payload map[string]any
		if unmarshalErr := json.Unmarshal(bodyBytes, &payload); unmarshalErr != nil {
			http.Error(responseWriter, "unmarshal error", http.StatusBadRequest)
			return
		}

		payloadDelivered <- payload
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	encryptedSigningSecret, err := cryptoKeyManager.EncryptField([]byte("webhook-secret-token"))
	if err != nil {
		t.Fatalf("failed to encrypt signing secret: %v", err)
	}

	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("webhook"),
		SenderEmail: "system@layr.sh",
		Webhook: EmailDispatcherWebhookConfig{
			URL:            webhookServer.URL,
			SigningSecret:  encryptedSigningSecret,
			TimeoutSeconds: 5,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)
	sendErr := emailDispatcher.SendEmailVerification(ctx, "verify-user@example.com", "777888", "user-uuid")
	if sendErr != nil {
		t.Fatalf("failed to dispatch email verification via webhook: %v", sendErr)
	}

	deliveredPayload := <-payloadDelivered
	if deliveredPayload["message_kind"] != "email_verification" {
		t.Fatalf("expected email_verification message_kind, got: %v", deliveredPayload["message_kind"])
	}
	if deliveredPayload["code"] != "777888" {
		t.Fatalf("expected code 777888, got: %v", deliveredPayload["code"])
	}
	if deliveredPayload["to"] != "verify-user@example.com" {
		t.Fatalf("expected recipient, got: %v", deliveredPayload["to"])
	}
}

func TestAuthEmailDeliveryFailureErrorE2E(t *testing.T) {
	ctx := context.Background()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	encryptedPassword, err := cryptoKeyManager.EncryptField([]byte("secret-password"))
	if err != nil {
		t.Fatalf("failed to encrypt password: %v", err)
	}

	// Unroutable blackhole TCP port to trigger dial failure
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver:      stringPointer("smtp"),
		SenderEmail: "system@layr.sh",
		SMTP: EmailDispatcherSMTPConfig{
			Host:               "127.0.0.1",
			Port:               1, // Privileged/closed port
			Username:           "layruser",
			Password:           encryptedPassword,
			TLSMode:            "none",
			InsecureSkipVerify: true,
		},
	}

	emailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig { return emailDispatcherConfig }, cryptoKeyManager)
	err = emailDispatcher.SendPasswordReset(ctx, "user@example.com", "123456", "user-uuid")
	if err == nil {
		t.Fatal("expected delivery failure error when SMTP server is unreachable")
	}
}
