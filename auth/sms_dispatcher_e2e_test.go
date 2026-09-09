package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthSMSUnconfiguredOTPSendE2E(t *testing.T) {
	ctx := context.Background()
	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return nil }, nil)

	err := smsDispatcher.SendSignInOTP(ctx, "+1234567890", "123456", "user-uuid")
	if err != ErrSMSDispatcherNotConfigured {
		t.Fatalf("expected ErrSMSDispatcherNotConfigured, got: %v", err)
	}
}

func TestAuthSMSSignInOTPDispatchTwilioE2E(t *testing.T) {
	ctx := context.Background()
	twilioPayloadReceived := make(chan map[string]string, 1)

	mockTwilioServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "AC1234567890" || password != "twilio-secret-token" {
			http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
			return
		}

		if parseErr := request.ParseForm(); parseErr != nil {
			http.Error(responseWriter, "parse error", http.StatusBadRequest)
			return
		}

		formData := map[string]string{
			"To":   request.FormValue("To"),
			"From": request.FormValue("From"),
			"Body": request.FormValue("Body"),
		}

		twilioPayloadReceived <- formData
		responseWriter.WriteHeader(http.StatusCreated)
		_, _ = responseWriter.Write([]byte(`{"sid":"SM123456","status":"queued"}`))
	}))
	defer mockTwilioServer.Close()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	encryptedAuthToken, err := cryptoKeyManager.EncryptField([]byte("twilio-secret-token"))
	if err != nil {
		t.Fatalf("failed to encrypt auth token: %v", err)
	}

	driverTwilio := "twilio"
	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: &driverTwilio,
		Twilio: SMSDispatcherTwilioConfig{
			AccountSID: "AC1234567890",
			AuthToken:  encryptedAuthToken,
			FromNumber: "+1987654321",
		},
	}

	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return smsDispatcherConfig }, cryptoKeyManager)
	smsDispatcher.httpClient = mockTwilioServer.Client()

	// Intercept Twilio default URL by pointing test to mock server URL
	mockTransport := &mockHostRewriteTransport{
		targetHost:   mockTwilioServer.Listener.Addr().String(),
		roundTripper: mockTwilioServer.Client().Transport,
	}
	smsDispatcher.httpClient.Transport = mockTransport

	sendErr := smsDispatcher.SendSignInOTP(ctx, "+1234567890", "654321", "user-uuid")
	if sendErr != nil {
		t.Fatalf("failed to dispatch twilio sms: %v", sendErr)
	}

	delivered := <-twilioPayloadReceived
	if delivered["To"] != "+1234567890" {
		t.Fatalf("expected recipient +1234567890, got: %s", delivered["To"])
	}
	if delivered["From"] != "+1987654321" {
		t.Fatalf("expected from +1987654321, got: %s", delivered["From"])
	}
	if !strings.Contains(delivered["Body"], "654321") {
		t.Fatalf("expected code in body, got: %s", delivered["Body"])
	}
}

func TestAuthSMSSignInOTPDispatchWebhookE2E(t *testing.T) {
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

	driverWebhook := "webhook"
	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL:            webhookServer.URL,
			SigningSecret:  encryptedSigningSecret,
			TimeoutSeconds: 5,
		},
	}

	smsDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig { return smsDispatcherConfig }, cryptoKeyManager)
	sendErr := smsDispatcher.SendPhoneVerification(ctx, "+1234567890", "888999", "user-uuid")
	if sendErr != nil {
		t.Fatalf("failed to dispatch phone verification via webhook: %v", sendErr)
	}

	deliveredPayload := <-payloadDelivered
	if deliveredPayload["message_kind"] != "phone_verification" {
		t.Fatalf("expected phone_verification message_kind, got: %v", deliveredPayload["message_kind"])
	}
	if deliveredPayload["code"] != "888999" {
		t.Fatalf("expected code 888999, got: %v", deliveredPayload["code"])
	}
	if deliveredPayload["to"] != "+1234567890" {
		t.Fatalf("expected recipient +1234567890, got: %v", deliveredPayload["to"])
	}
}

type mockHostRewriteTransport struct {
	targetHost   string
	roundTripper http.RoundTripper
}

func (transport *mockHostRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request.URL.Scheme = "http"
	request.URL.Host = transport.targetHost
	if transport.roundTripper != nil {
		response, err := transport.roundTripper.RoundTrip(request)
		if err != nil {
			return nil, fmt.Errorf("roundTripper transport roundtrip failed: %w", err)
		}
		return response, nil
	}
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("default transport roundtrip failed: %w", err)
	}
	return response, nil
}
