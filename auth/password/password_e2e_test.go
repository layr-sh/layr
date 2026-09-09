package password

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"uuid"
)

type testUser struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
}

type testAuthServer struct {
	rwMutex sync.RWMutex
	hasher  *Hasher
	users   map[string]*testUser // email -> user
}

func newTestAuthServer() *testAuthServer {
	return &testAuthServer{
		hasher: NewHasher(),
		users:  make(map[string]*testUser),
	}
}

func (server *testAuthServer) handleRegister(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	server.rwMutex.Lock()
	defer server.rwMutex.Unlock()

	if _, exists := server.users[requestBody.Email]; exists {
		http.Error(responseWriter, "user already exists", http.StatusConflict)
		return
	}

	passwordHash, err := server.hasher.Hash(requestBody.Password)
	if err != nil {
		http.Error(responseWriter, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &testUser{
		ID:           uuid.NewV7().String(),
		Email:        requestBody.Email,
		PasswordHash: passwordHash,
	}
	server.users[requestBody.Email] = user

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

func (server *testAuthServer) handleLogin(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	server.rwMutex.RLock()
	user, exists := server.users[requestBody.Email]
	server.rwMutex.RUnlock()

	if !exists {
		http.Error(responseWriter, "invalid credentials", http.StatusUnauthorized)
		return
	}

	isValid, err := server.hasher.Verify(requestBody.Password, user.PasswordHash)
	if err != nil || !isValid {
		http.Error(responseWriter, "invalid credentials", http.StatusUnauthorized)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"authenticated": true,
		"user_id":       user.ID,
		"session_token": uuid.NewV7().String(),
	})
}

func (server *testAuthServer) handleChangePassword(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody struct {
		Email       string `json:"email"`
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
		http.Error(responseWriter, "invalid json payload", http.StatusBadRequest)
		return
	}

	server.rwMutex.Lock()
	defer server.rwMutex.Unlock()

	user, exists := server.users[requestBody.Email]
	if !exists {
		http.Error(responseWriter, "user not found", http.StatusNotFound)
		return
	}

	isOldPasswordValid, err := server.hasher.Verify(requestBody.OldPassword, user.PasswordHash)
	if err != nil || !isOldPasswordValid {
		http.Error(responseWriter, "incorrect old password", http.StatusForbidden)
		return
	}

	newPasswordHash, err := server.hasher.Hash(requestBody.NewPassword)
	if err != nil {
		http.Error(responseWriter, "failed to hash new password", http.StatusInternalServerError)
		return
	}

	user.PasswordHash = newPasswordHash

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"updated": true,
	})
}

func TestPasswordAuthenticationFlowE2E(t *testing.T) {
	server := newTestAuthServer()

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/api/v1/auth/register", server.handleRegister)
	serveMux.HandleFunc("/api/v1/auth/login", server.handleLogin)
	serveMux.HandleFunc("/api/v1/auth/password/change", server.handleChangePassword)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	userEmail := "alice.password.test@example.com"
	initialPassword := "SuperStrongInitialPassword123!#"
	updatedPassword := "EvenStrongerRotatedPassword456!$"

	postJSON := func(endpointURL, payload string) (*http.Response, error) {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpointURL, strings.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := testServer.Client().Do(request)
		if err != nil {
			return nil, fmt.Errorf("failed to execute http request: %w", err)
		}
		return response, nil
	}

	// 1. User Registration
	registerPayload := `{"email":"` + userEmail + `","password":"` + initialPassword + `"}`
	registerResponse, err := postJSON(testServer.URL+"/api/v1/auth/register", registerPayload)
	if err != nil {
		t.Fatalf("failed to post registration: %v", err)
	}
	defer func() { _ = registerResponse.Body.Close() }()

	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created from registration, got: %d", registerResponse.StatusCode)
	}

	// 2. Login with Incorrect Password
	wrongLoginPayload := `{"email":"` + userEmail + `","password":"IncorrectPassword999!"}`
	wrongLoginResponse, err := postJSON(testServer.URL+"/api/v1/auth/login", wrongLoginPayload)
	if err != nil {
		t.Fatalf("failed to post login: %v", err)
	}
	defer func() { _ = wrongLoginResponse.Body.Close() }()

	if wrongLoginResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong password, got: %d", wrongLoginResponse.StatusCode)
	}

	// 3. Login with Correct Initial Password
	correctLoginPayload := `{"email":"` + userEmail + `","password":"` + initialPassword + `"}`
	correctLoginResponse, err := postJSON(testServer.URL+"/api/v1/auth/login", correctLoginPayload)
	if err != nil {
		t.Fatalf("failed to post valid login: %v", err)
	}
	defer func() { _ = correctLoginResponse.Body.Close() }()

	if correctLoginResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid login, got: %d", correctLoginResponse.StatusCode)
	}

	var sessionResult struct {
		Authenticated bool   `json:"authenticated"`
		UserID        string `json:"user_id"`
		SessionToken  string `json:"session_token"`
	}
	if decodeErr := json.NewDecoder(correctLoginResponse.Body).Decode(&sessionResult); decodeErr != nil {
		t.Fatalf("failed to decode login session response: %v", decodeErr)
	}
	if !sessionResult.Authenticated || sessionResult.SessionToken == "" {
		t.Fatalf("unexpected session response: %+v", sessionResult)
	}

	// 4. Change Password
	changePasswordPayload := `{"email":"` + userEmail + `","old_password":"` + initialPassword + `","new_password":"` + updatedPassword + `"}`
	changePasswordResponse, err := postJSON(testServer.URL+"/api/v1/auth/password/change", changePasswordPayload)
	if err != nil {
		t.Fatalf("failed to post password change: %v", err)
	}
	defer func() { _ = changePasswordResponse.Body.Close() }()

	if changePasswordResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from password change, got: %d", changePasswordResponse.StatusCode)
	}

	// 5. Old Password Login Fails After Change
	oldLoginResponse, err := postJSON(testServer.URL+"/api/v1/auth/login", correctLoginPayload)
	if err != nil {
		t.Fatalf("failed to post old login: %v", err)
	}
	defer func() { _ = oldLoginResponse.Body.Close() }()

	if oldLoginResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for obsolete password, got: %d", oldLoginResponse.StatusCode)
	}

	// 6. New Password Login Succeeds
	newLoginPayload := `{"email":"` + userEmail + `","password":"` + updatedPassword + `"}`
	newLoginResponse, err := postJSON(testServer.URL+"/api/v1/auth/login", newLoginPayload)
	if err != nil {
		t.Fatalf("failed to post new password login: %v", err)
	}
	defer func() { _ = newLoginResponse.Body.Close() }()

	if newLoginResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for new password login, got: %d", newLoginResponse.StatusCode)
	}
}
