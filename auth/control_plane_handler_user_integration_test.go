package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerUserIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	kvStore := newInMemoryKVStore()

	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Auth Control Plane Service Account",
		Scopes: []string{"*"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	configManager := NewConfigManager(db, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager)
	controlPlaneHandler.SetKVStore(kvStore)
	controlPlaneHandler.SetEventBus(eventBus)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	jwtSigner, signerErr := core.NewJWTSigner(cryptoKeyManager)
	if signerErr != nil {
		t.Fatalf("failed to create jwt signer: %v", signerErr)
	}
	controlPlaneHandler.SetJWTSigner(jwtSigner)

	authBearerHeader := "Bearer " + createdServiceAccount.SecretKey

	// 1. Create User via Control Plane
	createPayload := map[string]any{
		"email":          "cp-created@example.com",
		"password":       "ControlPlanePassword123!",
		"role":           "authenticated",
		"email_verified": true,
		"properties": map[string]any{
			"dept": "engineering",
		},
	}
	encodedCreate, _ := json.Marshal(createPayload)
	createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader(encodedCreate))
	createRequest.Header.Set("Authorization", authBearerHeader)
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(createResponseRecorder, createRequest)

	if createResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from HandleCreateUser, got: %d (body: %s)", createResponseRecorder.Code, createResponseRecorder.Body.String())
	}

	var createdUserRecord UserRecord
	if err := json.NewDecoder(createResponseRecorder.Body).Decode(&createdUserRecord); err != nil {
		t.Fatalf("failed to decode created user: %v", err)
	}

	// 2. Create User with Phone Only
	createPhonePayload := map[string]any{
		"phone":          "+19876543210",
		"phone_verified": true,
	}
	encodedCreatePhone, _ := json.Marshal(createPhonePayload)
	createPhoneRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader(encodedCreatePhone))
	createPhoneRequest.Header.Set("Authorization", authBearerHeader)
	createPhoneResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(createPhoneResponseRecorder, createPhoneRequest)
	if createPhoneResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on phone user: %d", createPhoneResponseRecorder.Code)
	}

	// 3. List Users with Pagination and Filters
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users?limit=10&offset=0&role=authenticated&search=cp-created", nil)
	listRequest.Header.Set("Authorization", authBearerHeader)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(listResponseRecorder, listRequest)

	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleListUsers, got: %d (body: %s)", listResponseRecorder.Code, listResponseRecorder.Body.String())
	}

	var listResponse struct {
		Users  []UserRecord `json:"users"`
		Limit  int          `json:"limit"`
		Offset int          `json:"offset"`
		Count  int          `json:"count"`
	}
	if err := json.NewDecoder(listResponseRecorder.Body).Decode(&listResponse); err != nil {
		t.Fatalf("failed to decode user list: %v", err)
	}
	if len(listResponse.Users) == 0 {
		t.Fatal("expected at least one user in user list")
	}

	// 4. Get User by ID
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+createdUserRecord.ID, nil)
	getRequest.Header.Set("Authorization", authBearerHeader)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(getResponseRecorder, getRequest)

	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleGetUser, got: %d (body: %s)", getResponseRecorder.Code, getResponseRecorder.Body.String())
	}

	// Get non-existent user returns 404
	randomID := uuid.NewV7().String()
	notFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+randomID, nil)
	notFoundRequest.Header.Set("Authorization", authBearerHeader)
	notFoundResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(notFoundResponseRecorder, notFoundRequest)
	if notFoundResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent user, got: %d", notFoundResponseRecorder.Code)
	}

	// 5. Lock and Unlock User
	_ = kvStore.Set(ctx, "auth:session:lock_hash", "cached_session", time.Hour)
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, client_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, 'client-lock-test', 'lock_hash', '127.0.0.1', 'Mozilla/5.0', clock_timestamp() + interval '30 days', clock_timestamp())
	`, createdUserRecord.ID)

	futureLockTime := time.Now().Add(48 * time.Hour).UTC()
	lockPayload, _ := json.Marshal(map[string]any{"locked_until": futureLockTime.Format(time.RFC3339)})
	lockRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+createdUserRecord.ID+"/lock", bytes.NewReader(lockPayload))
	lockRequest.Header.Set("Authorization", authBearerHeader)
	lockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(lockResponseRecorder, lockRequest)

	if lockResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleLockUser, got: %d", lockResponseRecorder.Code)
	}

	unlockRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+createdUserRecord.ID+"/lock", nil)
	unlockRequest.Header.Set("Authorization", authBearerHeader)
	unlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(unlockResponseRecorder, unlockRequest)

	if unlockResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleUnlockUser, got: %d", unlockResponseRecorder.Code)
	}

	// 6. Delete User
	deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+createdUserRecord.ID, nil)
	deleteRequest.Header.Set("Authorization", authBearerHeader)
	deleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(deleteResponseRecorder, deleteRequest)

	if deleteResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandleDeleteUser, got: %d", deleteResponseRecorder.Code)
	}

	// 7. Non-existent User Operations (Delete, Lock, Unlock -> 404)
	nonExistentUUID := uuid.NewV7().String()

	deleteNotFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+nonExistentUUID, nil)
	deleteNotFoundRequest.Header.Set("Authorization", authBearerHeader)
	deleteNotFoundResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(deleteNotFoundResponseRecorder, deleteNotFoundRequest)
	if deleteNotFoundResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on deleting non-existent user, got: %d", deleteNotFoundResponseRecorder.Code)
	}

	lockNotFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+nonExistentUUID+"/lock", strings.NewReader(`{}`))
	lockNotFoundRequest.Header.Set("Authorization", authBearerHeader)
	lockNotFoundResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(lockNotFoundResponseRecorder, lockNotFoundRequest)
	if lockNotFoundResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on locking non-existent user, got: %d", lockNotFoundResponseRecorder.Code)
	}

	unlockNotFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+nonExistentUUID+"/lock", nil)
	unlockNotFoundRequest.Header.Set("Authorization", authBearerHeader)
	unlockNotFoundResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(unlockNotFoundResponseRecorder, unlockNotFoundRequest)
	if unlockNotFoundResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unlocking non-existent user, got: %d", unlockNotFoundResponseRecorder.Code)
	}

	// 8. Create User with only Phone, and Create User with neither Email nor Phone
	noEmailOrPhonePayload, _ := json.Marshal(map[string]any{
		"role": "authenticated",
	})
	noEmailOrPhoneRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader(noEmailOrPhonePayload))
	noEmailOrPhoneRequest.Header.Set("Authorization", authBearerHeader)
	noEmailOrPhoneResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(noEmailOrPhoneResponseRecorder, noEmailOrPhoneRequest)
	if noEmailOrPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on creating user without email or phone, got: %d", noEmailOrPhoneResponseRecorder.Code)
	}

	uniquePhone := fmt.Sprintf("+1415%07d", time.Now().UnixNano()%10000000)
	phoneCreatePayload, _ := json.Marshal(map[string]any{
		"phone":          uniquePhone,
		"phone_verified": true,
		"role":           "authenticated",
	})
	phoneCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader(phoneCreatePayload))
	phoneCreateRequest.Header.Set("Authorization", authBearerHeader)
	phoneCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(phoneCreateResponseRecorder, phoneCreateRequest)
	if phoneCreateResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201 on creating user with phone only, got: %d (body: %s)", phoneCreateResponseRecorder.Code, phoneCreateResponseRecorder.Body.String())
	}

	// 9. List Users with Cursor pagination
	listCursorRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users?limit=1&cursor=2099-01-01T00:00:00Z", nil)
	listCursorRequest.Header.Set("Authorization", authBearerHeader)
	listCursorResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(listCursorResponseRecorder, listCursorRequest)
	if listCursorResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list users with cursor, got: %d", listCursorResponseRecorder.Code)
	}
}

func TestAuthControlPlaneHandlerUserBrokenPoolIntegration(t *testing.T) {
	brokenDB := createBrokenPool(t)
	if brokenDB == nil {
		t.Skip("skipping broken pool test")
		return
	}

	cryptoKeyManager, _ := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	configManager := NewConfigManager(brokenDB, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(brokenDB, configManager)
	randomID := uuid.NewV7().String()

	ctx := context.Background()

	// 1. List Users error
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool list, got: %d", listResponseRecorder.Code)
	}

	// 2. Create User error
	createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", strings.NewReader(`{"email":"broken@test.com"}`))
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(createResponseRecorder, createRequest)
	if createResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool create, got: %d", createResponseRecorder.Code)
	}

	// 3. Get User error
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+randomID, nil)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool get, got: %d", getResponseRecorder.Code)
	}

	// 4. Delete User error
	deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+randomID, nil)
	deleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool delete, got: %d", deleteResponseRecorder.Code)
	}

	// 5. Lock User error
	lockRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+randomID+"/lock", nil)
	lockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(lockResponseRecorder, lockRequest)
	if lockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool lock, got: %d", lockResponseRecorder.Code)
	}

	// 6. Unlock User error
	unlockRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/auth/users/"+randomID+"/lock", nil)
	unlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(unlockResponseRecorder, unlockRequest)
	if unlockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool unlock, got: %d", unlockResponseRecorder.Code)
	}
}
