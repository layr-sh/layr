package passkey

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"uuid"
)

type simulatedPasskeyService struct {
	rwMutex     sync.RWMutex
	manager     *Manager
	credentials map[string]*Credential // id -> Credential
}

func newSimulatedPasskeyService(relyingPartyID, relyingPartyName string) *simulatedPasskeyService {
	return &simulatedPasskeyService{
		manager:     NewManager(relyingPartyID, relyingPartyName),
		credentials: make(map[string]*Credential),
	}
}

func (service *simulatedPasskeyService) handleRegisterOptions(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	signUpOptions, err := service.manager.BeginSignUp(requestBody.UserID, requestBody.UserName)
	if err != nil {
		http.Error(responseWriter, "failed to begin sign up", http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signUpOptions)
}

func (service *simulatedPasskeyService) handleRegisterVerify(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Challenge    string   `json:"challenge"`
		CredentialID string   `json:"credential_id"`
		PublicKey    string   `json:"public_key"`
		FriendlyName string   `json:"friendly_name"`
		Transports   []string `json:"transports"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	userID, err := service.manager.ConsumeChallenge(requestBody.Challenge)
	if err != nil {
		http.Error(responseWriter, "invalid challenge: "+err.Error(), http.StatusBadRequest)
		return
	}

	rawCredentialID, _ := base64.RawURLEncoding.DecodeString(requestBody.CredentialID)
	rawPublicKey, _ := base64.RawURLEncoding.DecodeString(requestBody.PublicKey)

	credential := &Credential{
		ID:           uuid.NewV7().String(),
		UserID:       userID,
		CredentialID: rawCredentialID,
		PublicKey:    rawPublicKey,
		Counter:      0,
		Transports:   requestBody.Transports,
		FriendlyName: requestBody.FriendlyName,
	}

	service.rwMutex.Lock()
	service.credentials[requestBody.CredentialID] = credential
	service.rwMutex.Unlock()

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{"registered": true, "credential_id": credential.ID})
}

func (service *simulatedPasskeyService) handleSignInOptions(responseWriter http.ResponseWriter, _ *http.Request) {
	signInOptions, err := service.manager.BeginSignIn()
	if err != nil {
		http.Error(responseWriter, "failed to begin signin", http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signInOptions)
}

func (service *simulatedPasskeyService) handleSignInVerify(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Challenge         string `json:"challenge"`
		CredentialID      string `json:"credential_id"`
		ClientDataJSON    string `json:"client_data_json"`
		AuthenticatorData string `json:"authenticator_data"`
		Signature         string `json:"signature"`
		Counter           uint32 `json:"counter"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	_, err := service.manager.ConsumeChallenge(requestBody.Challenge)
	if err != nil {
		http.Error(responseWriter, "invalid challenge: "+err.Error(), http.StatusBadRequest)
		return
	}

	service.rwMutex.Lock()
	credential, exists := service.credentials[requestBody.CredentialID]
	service.rwMutex.Unlock()

	if !exists {
		http.Error(responseWriter, "credential not found", http.StatusNotFound)
		return
	}

	// Monotonic Counter check (replay attack protection)
	if requestBody.Counter <= credential.Counter {
		http.Error(responseWriter, "potential cloned authenticator detected: counter did not advance", http.StatusForbidden)
		return
	}

	rawClientData, _ := base64.RawURLEncoding.DecodeString(requestBody.ClientDataJSON)
	rawAuthenticatorData, _ := base64.RawURLEncoding.DecodeString(requestBody.AuthenticatorData)
	rawSignature, _ := base64.RawURLEncoding.DecodeString(requestBody.Signature)

	if !VerifySignature(credential.PublicKey, rawClientData, rawAuthenticatorData, rawSignature) {
		http.Error(responseWriter, "invalid passkey signature", http.StatusUnauthorized)
		return
	}

	service.rwMutex.Lock()
	credential.Counter = requestBody.Counter
	service.rwMutex.Unlock()

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"authenticated": true,
		"user_id":       credential.UserID,
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

func TestPasskeySignUpAndAuthenticationFlowE2E(t *testing.T) {
	service := newSimulatedPasskeyService("app.layr.sh", "Layr App")

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/api/v1/auth/passkey/register/options", service.handleRegisterOptions)
	serveMux.HandleFunc("/api/v1/auth/passkey/register/verify", service.handleRegisterVerify)
	serveMux.HandleFunc("/api/v1/auth/passkey/signin/options", service.handleSignInOptions)
	serveMux.HandleFunc("/api/v1/auth/passkey/signin/verify", service.handleSignInVerify)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	testClient := testServer.Client()
	ctx := context.Background()
	applicationUserID := uuid.NewV7().String()
	userName := "AlicePasskey"

	// 1. Begin Sign Up (Fetch Options)
	registerOptionsPayload := `{"user_id":"` + applicationUserID + `","user_name":"` + userName + `"}`
	signUpOptionsResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/register/options", registerOptionsPayload)
	defer func() { _ = signUpOptionsResponse.Body.Close() }()

	if signUpOptionsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from register options, got: %d", signUpOptionsResponse.StatusCode)
	}

	var signUpOptions SignUpOptions
	if decodeErr := json.NewDecoder(signUpOptionsResponse.Body).Decode(&signUpOptions); decodeErr != nil {
		t.Fatalf("failed to decode sign up options: %v", decodeErr)
	}

	if signUpOptions.Challenge == "" || signUpOptions.RelyingPartyID != "app.layr.sh" {
		t.Fatalf("unexpected sign up options: %+v", signUpOptions)
	}

	// 2. Client Authenticator Sign Up (Simulate WebAuthn create)
	clientCredentialID := base64.RawURLEncoding.EncodeToString([]byte("cred-unique-raw-bytes-1001"))
	clientPublicKey := base64.RawURLEncoding.EncodeToString([]byte("pubkey-es256-authenticator-bytes"))

	registerVerifyPayload := map[string]any{
		"challenge":     signUpOptions.Challenge,
		"credential_id": clientCredentialID,
		"public_key":    clientPublicKey,
		"friendly_name": "TouchID MacBook Pro",
		"transports":    []string{"internal"},
	}
	encodedRegisterVerify, marshalErr := json.Marshal(registerVerifyPayload)
	if marshalErr != nil {
		t.Fatalf("failed to marshal register verify payload: %v", marshalErr)
	}

	signUpVerifyResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/register/verify", string(encodedRegisterVerify))
	defer func() { _ = signUpVerifyResponse.Body.Close() }()

	if signUpVerifyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from register verify, got: %d", signUpVerifyResponse.StatusCode)
	}

	// 3. Begin Sign-In (Fetch Sign-In Options)
	signInOptionsResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/signin/options", `{}`)
	defer func() { _ = signInOptionsResponse.Body.Close() }()

	var signInOptions SignInOptions
	if decodeErr := json.NewDecoder(signInOptionsResponse.Body).Decode(&signInOptions); decodeErr != nil {
		t.Fatalf("failed to decode signin options: %v", decodeErr)
	}

	// 4. Client Assertion & Sign-In Verification (Simulate WebAuthn get)
	clientDataJSON := base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get","challenge":"` + signInOptions.Challenge + `"}`))
	authenticatorData := base64.RawURLEncoding.EncodeToString([]byte("authdata-flags-counter"))
	assertionSignature := base64.RawURLEncoding.EncodeToString([]byte("valid-assertion-signature"))

	signInVerifyPayload := map[string]any{
		"challenge":          signInOptions.Challenge,
		"credential_id":      clientCredentialID,
		"client_data_json":   clientDataJSON,
		"authenticator_data": authenticatorData,
		"signature":          assertionSignature,
		"counter":            1,
	}
	encodedSignInVerify, marshalSignErr := json.Marshal(signInVerifyPayload)
	if marshalSignErr != nil {
		t.Fatalf("failed to marshal signin verify payload: %v", marshalSignErr)
	}

	signInVerifyResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/signin/verify", string(encodedSignInVerify))
	defer func() { _ = signInVerifyResponse.Body.Close() }()

	if signInVerifyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from signin verify, got: %d", signInVerifyResponse.StatusCode)
	}

	var sessionResult struct {
		Authenticated bool   `json:"authenticated"`
		UserID        string `json:"user_id"`
		SessionToken  string `json:"session_token"`
	}
	if decodeErr := json.NewDecoder(signInVerifyResponse.Body).Decode(&sessionResult); decodeErr != nil {
		t.Fatalf("failed to decode session result: %v", decodeErr)
	}
	if !sessionResult.Authenticated || sessionResult.UserID != applicationUserID || !strings.HasPrefix(sessionResult.SessionToken, "sess_live_") {
		t.Fatalf("unexpected session result: %+v", sessionResult)
	}

	// 5. Replay Attack Prevention (Replaying same counter or challenge)
	// Replay challenge (already consumed)
	replayChallengeResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/signin/verify", string(encodedSignInVerify))
	defer func() { _ = replayChallengeResponse.Body.Close() }()

	if replayChallengeResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on consumed challenge replay, got: %d", replayChallengeResponse.StatusCode)
	}

	// 6. Monotonic Counter Regression / Clone Detection
	// Issue a fresh challenge, but client submits a stale/lower counter (counter = 1, current stored counter is 1)
	freshSignInOptionsResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/signin/options", `{}`)
	defer func() { _ = freshSignInOptionsResponse.Body.Close() }()

	var freshSignInOptions SignInOptions
	_ = json.NewDecoder(freshSignInOptionsResponse.Body).Decode(&freshSignInOptions)

	staleCounterPayload := map[string]any{
		"challenge":          freshSignInOptions.Challenge,
		"credential_id":      clientCredentialID,
		"client_data_json":   clientDataJSON,
		"authenticator_data": authenticatorData,
		"signature":          assertionSignature,
		"counter":            1, // Must be > 1 to succeed!
	}
	encodedStaleCounter, marshalStaleErr := json.Marshal(staleCounterPayload)
	if marshalStaleErr != nil {
		t.Fatalf("failed to marshal stale counter payload: %v", marshalStaleErr)
	}

	staleCounterResponse := executePostRequest(ctx, t, testClient, testServer.URL+"/api/v1/auth/passkey/signin/verify", string(encodedStaleCounter))
	defer func() { _ = staleCounterResponse.Body.Close() }()

	if staleCounterResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on stale counter, got: %d", staleCounterResponse.StatusCode)
	}
}
