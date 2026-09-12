package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func TestAuthEmailVerificationFullLifecycleE2E(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("POST /api/v1/auth/user/email/verification/request", baseHandler.handleUserEmailVerificationRequest)
	serveMux.HandleFunc("POST /api/v1/auth/user/email/verification/confirm", baseHandler.handleUserEmailVerificationConfirm)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()
	httpClient := testServer.Client()

	// 1. Seed user in database
	userEmail := "e2e-email-verify@example.com"
	insertUserQuery := `
		INSERT INTO auth.users (id, email, role, email_verified_at, created_at, last_updated_at)
		VALUES (uuidv7(), $1, 'user', NULL, clock_timestamp(), clock_timestamp())
	`
	if _, err := db.Exec(ctx, insertUserQuery, userEmail); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// 2. Mock Email Webhook Server
	dispatchedCodeChannel := make(chan string, 1)
	mockEmailWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if codeValue, ok := payload["code"].(string); ok {
			dispatchedCodeChannel <- codeValue
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mockEmailWebhookServer.Close()

	// Update dynamic config with webhook email driver
	driverWebhook := "webhook"
	activeConfig := configManager.Get()
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:      &driverWebhook,
		SenderEmail: "security@layr.sh",
		Webhook: EmailDispatcherWebhookConfig{
			URL:            mockEmailWebhookServer.URL,
			TimeoutSeconds: 5,
		},
	}
	configManager.Set(activeConfig)

	// 3. Request Email Verification
	verifyRequestPayload, _ := json.Marshal(map[string]any{
		"email": userEmail,
	})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/email/verification/request", bytes.NewReader(verifyRequestPayload))
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatalf("failed to send verify request: %v", err)
	}
	_ = response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on verify request, got: %d", response.StatusCode)
	}

	var interceptedCode string
	select {
	case interceptedCode = <-dispatchedCodeChannel:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email verification webhook dispatch")
	}

	const expectedCodeLength = 6
	if len(interceptedCode) != expectedCodeLength {
		t.Fatalf("expected 6-digit verification code, got: %s", interceptedCode)
	}

	// 4. Confirm Email Verification
	confirmPayload, _ := json.Marshal(map[string]any{
		"email": userEmail,
		"code":  interceptedCode,
	})
	confirmRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/email/verification/confirm", bytes.NewReader(confirmPayload))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmResponse, err := httpClient.Do(confirmRequest)
	if err != nil {
		t.Fatalf("failed to send confirm request: %v", err)
	}
	defer func() { _ = confirmResponse.Body.Close() }()

	if confirmResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on verify confirm, got: %d", confirmResponse.StatusCode)
	}

	// 5. Assert user email_verified_at is set in database
	var emailVerifiedAt *time.Time
	queryErr := db.QueryRow(ctx, "SELECT email_verified_at FROM auth.users WHERE email = $1", userEmail).Scan(&emailVerifiedAt)
	if queryErr != nil {
		t.Fatalf("failed to query user email verification status: %v", queryErr)
	}
	if emailVerifiedAt == nil {
		t.Fatal("expected email_verified_at to be populated in database")
	}
}

func TestAuthPhoneVerificationFullLifecycleE2E(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("POST /api/v1/auth/user/phone/verification/request", baseHandler.handleUserPhoneVerificationRequest)
	serveMux.HandleFunc("POST /api/v1/auth/user/phone/verification/confirm", baseHandler.handleUserPhoneVerificationConfirm)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()
	httpClient := testServer.Client()

	// Create user with phone
	testPhoneNumber := "+15550009999"
	insertUserQuery := `
		INSERT INTO auth.users (id, email, phone, role, email_verified_at, phone_verified_at, created_at, last_updated_at)
		VALUES (uuidv7(), 'phone-e2e@example.com', $1, 'user', NULL, NULL, clock_timestamp(), clock_timestamp())
	`
	if _, err := db.Exec(ctx, insertUserQuery, testPhoneNumber); err != nil {
		t.Fatalf("failed to insert user with phone: %v", err)
	}

	// Mock SMS Webhook Server
	dispatchedSMSChannel := make(chan string, 1)
	mockSMSWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if codeValue, ok := payload["code"].(string); ok {
			dispatchedSMSChannel <- codeValue
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mockSMSWebhookServer.Close()

	// Update dynamic config with SMS webhook driver
	driverWebhookSMS := "webhook"
	activeConfig := configManager.Get()
	activeConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhookSMS,
		Webhook: SMSDispatcherWebhookConfig{
			URL:            mockSMSWebhookServer.URL,
			TimeoutSeconds: 5,
		},
	}
	configManager.Set(activeConfig)

	// 1. Request Phone Verification
	verifyRequestPayload, _ := json.Marshal(map[string]any{
		"phone": testPhoneNumber,
	})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/phone/verification/request", bytes.NewReader(verifyRequestPayload))
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatalf("failed to send phone verify request: %v", err)
	}
	_ = response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on phone verify request, got: %d", response.StatusCode)
	}

	var interceptedCode string
	select {
	case interceptedCode = <-dispatchedSMSChannel:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SMS phone verification webhook dispatch")
	}

	const expectedCodeLength = 6
	if len(interceptedCode) != expectedCodeLength {
		t.Fatalf("expected 6-digit phone verification code, got: %s", interceptedCode)
	}

	// 2. Confirm Phone Verification
	confirmPayload, _ := json.Marshal(map[string]any{
		"phone": testPhoneNumber,
		"code":  interceptedCode,
	})
	confirmRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/phone/verification/confirm", bytes.NewReader(confirmPayload))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmResponse, err := httpClient.Do(confirmRequest)
	if err != nil {
		t.Fatalf("failed to send phone confirm request: %v", err)
	}
	defer func() { _ = confirmResponse.Body.Close() }()

	if confirmResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on phone verify confirm, got: %d", confirmResponse.StatusCode)
	}

	// 3. Assert user phone_verified_at is set in database
	var phoneVerifiedAt *time.Time
	queryErr := db.QueryRow(ctx, "SELECT phone_verified_at FROM auth.users WHERE phone = $1", testPhoneNumber).Scan(&phoneVerifiedAt)
	if queryErr != nil {
		t.Fatalf("failed to query user phone verification status: %v", queryErr)
	}
	if phoneVerifiedAt == nil {
		t.Fatal("expected phone_verified_at to be populated in database")
	}
}

func TestAuthVerificationUnconfiguredE2E(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("POST /api/v1/auth/user/email/verification/request", baseHandler.handleUserEmailVerificationRequest)
	serveMux.HandleFunc("POST /api/v1/auth/user/phone/verification/request", baseHandler.handleUserPhoneVerificationRequest)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()
	httpClient := testServer.Client()

	// Ensure config has unconfigured driver (nil) for Email and SMS
	activeConfig := configManager.Get()
	activeConfig.EmailDispatcher.Driver = nil
	activeConfig.SMSDispatcher.Driver = nil
	configManager.Set(activeConfig)

	// 1. Email Verification Request -> 422 Unprocessable Entity
	emailPayload, _ := json.Marshal(map[string]any{"email": "unconfigured@example.com"})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/email/verification/request", bytes.NewReader(emailPayload))
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatalf("failed to send unconfigured email request: %v", err)
	}
	bodyBytes, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on email request, got: %d (%s)", response.StatusCode, string(bodyBytes))
	}
	if !strings.Contains(string(bodyBytes), "LAYR_AUTH_EMAIL_UNAVAILABLE") {
		t.Fatalf("expected LAYR_AUTH_EMAIL_UNAVAILABLE in response, got: %s", string(bodyBytes))
	}

	// 2. Phone Verification Request -> 422 Unprocessable Entity
	phonePayload, _ := json.Marshal(map[string]any{"phone": "+15551112222"})
	phoneRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, testServer.URL+"/api/v1/auth/user/phone/verification/request", bytes.NewReader(phonePayload))
	phoneRequest.Header.Set("Content-Type", "application/json")
	phoneResponse, err := httpClient.Do(phoneRequest)
	if err != nil {
		t.Fatalf("failed to send unconfigured phone request: %v", err)
	}
	phoneBodyBytes, _ := io.ReadAll(phoneResponse.Body)
	_ = phoneResponse.Body.Close()

	if phoneResponse.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on phone request, got: %d (%s)", phoneResponse.StatusCode, string(phoneBodyBytes))
	}
	if !strings.Contains(string(phoneBodyBytes), "LAYR_AUTH_SMS_UNAVAILABLE") {
		t.Fatalf("expected LAYR_AUTH_SMS_UNAVAILABLE in response, got: %s", string(phoneBodyBytes))
	}
}
