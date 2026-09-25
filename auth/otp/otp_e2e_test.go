package otp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"
)

type simulatedOTPDispatcher struct {
	mutex    sync.Mutex
	messages map[string]string
}

func newSimulatedOTPDispatcher() *simulatedOTPDispatcher {
	return &simulatedOTPDispatcher{
		messages: make(map[string]string),
	}
}

func (dispatcher *simulatedOTPDispatcher) Send(recipient, message string) {
	dispatcher.mutex.Lock()
	defer dispatcher.mutex.Unlock()
	dispatcher.messages[recipient] = message
}

func (dispatcher *simulatedOTPDispatcher) LastMessage(recipient string) string {
	dispatcher.mutex.Lock()
	defer dispatcher.mutex.Unlock()
	return dispatcher.messages[recipient]
}

type simulatedAuthServer struct {
	mutex      sync.Mutex
	records    map[string]*Record
	dispatcher *simulatedOTPDispatcher
}

func newSimulatedAuthServer(dispatcher *simulatedOTPDispatcher) *simulatedAuthServer {
	return &simulatedAuthServer{
		records:    make(map[string]*Record),
		dispatcher: dispatcher,
	}
}

func (server *simulatedAuthServer) handleSend(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Recipient string `json:"recipient"`
		Purpose   string `json:"purpose"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	code, err := GenerateCode(nil)
	if err != nil {
		http.Error(responseWriter, "failed to generate code", http.StatusInternalServerError)
		return
	}

	record := &Record{
		ID:        uuid.NewV7().String(),
		Recipient: requestBody.Recipient,
		CodeHash:  HashCode(code),
		Purpose:   requestBody.Purpose,
		Attempts:  0,
		ExpiresAt: time.Now().UTC().Add(CodeTTL),
		CreatedAt: time.Now().UTC(),
	}

	server.mutex.Lock()
	server.records[requestBody.Recipient] = record
	server.mutex.Unlock()

	server.dispatcher.Send(requestBody.Recipient, code)

	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"status": "code_sent"})
}

func (server *simulatedAuthServer) handleVerify(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Recipient string `json:"recipient"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	server.mutex.Lock()
	record, exists := server.records[requestBody.Recipient]
	server.mutex.Unlock()

	if !exists {
		http.Error(responseWriter, "no active OTP for recipient", http.StatusNotFound)
		return
	}

	if IsExpired(record.ExpiresAt) {
		http.Error(responseWriter, "otp code expired", http.StatusBadRequest)
		return
	}

	if record.Attempts >= MaxAttempts {
		http.Error(responseWriter, "maximum verification attempts exceeded", http.StatusTooManyRequests)
		return
	}

	if !VerifyCode(requestBody.Code, record.CodeHash) {
		server.mutex.Lock()
		record.Attempts++
		server.mutex.Unlock()
		http.Error(responseWriter, "invalid otp code", http.StatusUnauthorized)
		return
	}

	// Code is valid: consume record and issue session
	server.mutex.Lock()
	delete(server.records, requestBody.Recipient)
	server.mutex.Unlock()

	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"authenticated": true,
		"session_token": "sess_live_" + uuid.NewV7().String(),
	})
}

func executePostRequest(ctx context.Context, t *testing.T, client *http.Client, endpointURL, payload string) *http.Response {
	t.Helper()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := client.Do(httpRequest)
	if err != nil {
		t.Fatalf("failed to execute post request to %s: %v", endpointURL, err)
	}
	return response
}

func TestOtpAuthenticationFlowE2E(t *testing.T) {
	dispatcher := newSimulatedOTPDispatcher()
	authServer := newSimulatedAuthServer(dispatcher)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/v1/auth/otp/send", authServer.handleSend)
	serveMux.HandleFunc("/v1/auth/otp/verify", authServer.handleVerify)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	testClient := testServer.Client()
	ctx := context.Background()
	userPhoneNumber := "+15551239876"

	// 1. Send OTP Request
	sendPayload := `{"recipient":"` + userPhoneNumber + `","purpose":"login"}`
	sendResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/send", sendPayload)
	defer func() { _ = sendResponse.Body.Close() }()

	if sendResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from send endpoint, got: %d", sendResponse.StatusCode)
	}

	// 2. Intercept dispatched code
	receivedCode := dispatcher.LastMessage(userPhoneNumber)
	if len(receivedCode) != expectedCodeLength {
		t.Fatalf("expected 6-digit dispatched code, got: %s", receivedCode)
	}

	// 3. Brute-force lockout scenario
	// Attacker sends 5 wrong codes
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		verifyPayload := `{"recipient":"` + userPhoneNumber + `","code":"000000"}`
		if receivedCode == "000000" {
			verifyPayload = `{"recipient":"` + userPhoneNumber + `","code":"111111"}`
		}
		failedResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/verify", verifyPayload)
		_ = failedResponse.Body.Close()

		if failedResponse.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for bad attempt %d, got: %d", attempt, failedResponse.StatusCode)
		}
	}

	// 6th attempt with correct code should be locked out (429 Too Many Requests)
	lockedOutPayload := `{"recipient":"` + userPhoneNumber + `","code":"` + receivedCode + `"}`
	lockedOutResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/verify", lockedOutPayload)
	defer func() { _ = lockedOutResponse.Body.Close() }()

	if lockedOutResponse.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on locked out token, got: %d", lockedOutResponse.StatusCode)
	}

	// 4. Successful login flow with fresh code
	sendFreshResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/send", sendPayload)
	_ = sendFreshResponse.Body.Close()

	freshCode := dispatcher.LastMessage(userPhoneNumber)
	if len(freshCode) != expectedCodeLength {
		t.Fatalf("expected 6-digit fresh code, got: %s", freshCode)
	}

	successfulPayload := `{"recipient":"` + userPhoneNumber + `","code":"` + freshCode + `"}`
	successfulResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/verify", successfulPayload)
	defer func() { _ = successfulResponse.Body.Close() }()

	if successfulResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on valid verification, got: %d", successfulResponse.StatusCode)
	}

	var sessionResult struct {
		Authenticated bool   `json:"authenticated"`
		SessionToken  string `json:"session_token"`
	}
	if decodeErr := json.NewDecoder(successfulResponse.Body).Decode(&sessionResult); decodeErr != nil {
		t.Fatalf("failed to decode session response: %v", decodeErr)
	}
	if !sessionResult.Authenticated || !strings.HasPrefix(sessionResult.SessionToken, "sess_live_") {
		t.Fatalf("unexpected session result: %+v", sessionResult)
	}

	// 5. Consumed code replay prevention
	replayResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/verify", successfulPayload)
	defer func() { _ = replayResponse.Body.Close() }()

	if replayResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on consumed replay code, got: %d", replayResponse.StatusCode)
	}

	// 6. Expired code rejection
	sendExpiredResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/send", sendPayload)
	_ = sendExpiredResponse.Body.Close()

	expiredCode := dispatcher.LastMessage(userPhoneNumber)
	// Artificially expire the record in server store
	authServer.mutex.Lock()
	authServer.records[userPhoneNumber].ExpiresAt = time.Now().UTC().Add(-1 * time.Minute)
	authServer.mutex.Unlock()

	expiredPayload := `{"recipient":"` + userPhoneNumber + `","code":"` + expiredCode + `"}`
	verifyExpiredResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/v1/auth/otp/verify", expiredPayload)
	defer func() { _ = verifyExpiredResponse.Body.Close() }()

	if verifyExpiredResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on expired code, got: %d", verifyExpiredResponse.StatusCode)
	}
}
