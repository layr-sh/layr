package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"layr.sh/core"
)

func TestAuthConfigManagerDatabaseIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)

	// 1. Initial Load creates and saves default config
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}

	loadedConfig := configManager.Get()
	if loadedConfig.Sessions.AccessTokenExpirySeconds != 900 {
		t.Fatalf("expected 900s access token expiry, got: %d", loadedConfig.Sessions.AccessTokenExpirySeconds)
	}

	// 2. Modify and Save config
	updatedConfig := loadedConfig
	updatedConfig.Password.MinLength = 12
	updatedConfig.Sessions.AccessTokenExpirySeconds = 1800
	if err := configManager.Save(ctx, updatedConfig); err != nil {
		t.Fatalf("failed to save updated config: %v", err)
	}

	// 3. Create second manager and assert persisted values load accurately
	secondaryConfigManager := NewConfigManager(db, cryptoKeyManager)
	if err := secondaryConfigManager.Load(ctx); err != nil {
		t.Fatalf("failed to load secondary manager: %v", err)
	}
	if secondaryConfigManager.Get().Password.MinLength != 12 || secondaryConfigManager.Get().Sessions.AccessTokenExpirySeconds != 1800 {
		t.Fatalf("mismatched loaded secondary config: %+v", secondaryConfigManager.Get())
	}

	// 4. HandlePutConfig with plaintext secrets and event bus
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	configManager.SetEventBus(eventBus)

	var mutex sync.Mutex
	var receivedEvent core.Event
	eventBus.Subscribe("auth.config.updated", func(eventCtx context.Context, event core.Event) error {
		mutex.Lock()
		defer mutex.Unlock()
		receivedEvent = event
		return nil
	})

	putPayload := map[string]any{
		"methods": map[string]any{
			"password": map[string]any{
				"enabled":    true,
				"min_length": 14,
			},
		},
		"oauth_providers": map[string]any{
			"google": map[string]any{
				"enabled":       true,
				"client_id":     "google-id-123",
				"client_secret": "my-plain-google-secret",
			},
		},
		"email_dispatcher": map[string]any{
			"smtp": map[string]any{
				"password": "my-plain-smtp-password",
			},
		},
	}
	encodedPutPayload, _ := json.Marshal(putPayload)

	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/auth/config", bytes.NewReader(encodedPutPayload))
	putResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(putResponseRecorder, putRequest)

	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from HandlePutConfig, got: %d (body: %s)", putResponseRecorder.Code, putResponseRecorder.Body.String())
	}

	// Verify sanitized secrets in PUT response
	if !strings.Contains(putResponseRecorder.Body.String(), `"client_secret_configured":true`) || !strings.Contains(putResponseRecorder.Body.String(), `"password_configured":true`) {
		t.Fatalf("expected configured flags in PUT response: %s", putResponseRecorder.Body.String())
	}
	if strings.Contains(putResponseRecorder.Body.String(), "my-plain-google-secret") || strings.Contains(putResponseRecorder.Body.String(), "my-plain-smtp-password") {
		t.Fatalf("plaintext secrets leaked in PUT response: %s", putResponseRecorder.Body.String())
	}

	// Verify database contains envelope-encrypted secrets
	var rawJSONInDB []byte
	err := db.QueryRow(ctx, "SELECT value FROM layr_auth.config WHERE key = $1", ConfigKey).Scan(&rawJSONInDB)
	if err != nil {
		t.Fatalf("failed to query config from db: %v", err)
	}

	var dbStoredConfig Config
	_ = json.Unmarshal(rawJSONInDB, &dbStoredConfig)
	if !strings.HasPrefix(dbStoredConfig.OAuthProviders["google"].ClientSecret, "enc:v1:") {
		t.Fatalf("expected encrypted Google secret in DB, got: %s", dbStoredConfig.OAuthProviders["google"].ClientSecret)
	}
	if !strings.HasPrefix(dbStoredConfig.EmailDispatcher.SMTP.Password, "enc:v1:") {
		t.Fatalf("expected encrypted SMTP password in DB, got: %+v", dbStoredConfig.EmailDispatcher)
	}

	// Allow event bus propagation
	time.Sleep(50 * time.Millisecond)
	mutex.Lock()
	capturedEvent := receivedEvent
	mutex.Unlock()
	if capturedEvent.Type != "auth.config.updated" {
		t.Fatalf("expected auth.config.updated event, got: %+v", capturedEvent)
	}

	// 5. Preserving existing secrets when omitted in subsequent PUT
	secondPutPayload := map[string]any{
		"methods": map[string]any{
			"password": map[string]any{
				"enabled":    true,
				"min_length": 16,
			},
		},
		"oauth_providers": map[string]any{
			"google": map[string]any{
				"enabled":   true,
				"client_id": "google-id-123",
			},
		},
		"smtp": map[string]any{
			"enabled": true,
		},
	}
	encodedSecondPut, _ := json.Marshal(secondPutPayload)

	secondPutRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/auth/config", bytes.NewReader(encodedSecondPut))
	secondPutResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(secondPutResponseRecorder, secondPutRequest)

	if secondPutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from second PUT, got: %d", secondPutResponseRecorder.Code)
	}
	if !strings.Contains(secondPutResponseRecorder.Body.String(), `"client_secret_configured":true`) {
		t.Fatalf("expected preserved secret configured flag: %s", secondPutResponseRecorder.Body.String())
	}

	// 5b. Resetting email and sms driver to null again after set
	nullDriverPayload := map[string]any{
		"email_dispatcher": map[string]any{
			"driver": nil,
		},
		"sms_dispatcher": map[string]any{
			"driver": nil,
		},
	}
	encodedNullDriver, _ := json.Marshal(nullDriverPayload)
	nullDriverRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/auth/config", bytes.NewReader(encodedNullDriver))
	nullDriverResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(nullDriverResponseRecorder, nullDriverRequest)

	if nullDriverResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from null driver PUT, got: %d", nullDriverResponseRecorder.Code)
	}

	persistedConfig := configManager.Get()
	if persistedConfig.EmailDispatcher.Driver != nil {
		t.Fatalf("expected email driver to be nil after reset, got: %v", *persistedConfig.EmailDispatcher.Driver)
	}
	if persistedConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected sms driver to be nil after reset, got: %v", *persistedConfig.SMSDispatcher.Driver)
	}

	// Verify database persistence of null driver
	var rawNullDriverJSON []byte
	if err := db.QueryRow(ctx, "SELECT value FROM layr_auth.config WHERE key = $1", ConfigKey).Scan(&rawNullDriverJSON); err != nil {
		t.Fatalf("failed to query config from db: %v", err)
	}
	var nullDriverConfig Config
	_ = json.Unmarshal(rawNullDriverJSON, &nullDriverConfig)
	if nullDriverConfig.EmailDispatcher.Driver != nil || nullDriverConfig.SMSDispatcher.Driver != nil {
		t.Fatalf("expected null driver in database JSON, got email: %v, sms: %v", nullDriverConfig.EmailDispatcher.Driver, nullDriverConfig.SMSDispatcher.Driver)
	}

	// 6. Test invalid JSON in DB table
	_, _ = db.Exec(ctx, "UPDATE layr_auth.config SET value = '123' WHERE key = $1", ConfigKey)
	if err := configManager.Load(ctx); err == nil {
		t.Fatal("expected error on corrupted JSON in database")
	}
}

func TestAuthConfigManagerScopeEnforcementIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)

	serviceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only Service Account",
		Scopes: []string{"auth:config.read"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	configManager := NewConfigManager(db, cryptoKeyManager)
	configManager.SetServiceAccountManager(serviceAccountManager)

	// 1. GET with read scope succeeds
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/config", nil)
	getRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	getResponseRecorder := httptest.NewRecorder()
	configManager.HandleGetConfig(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET with read scope, got: %d", getResponseRecorder.Code)
	}

	// 2. PUT with only read scope fails with 403 Forbidden
	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(`{}`))
	putRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	putResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(putResponseRecorder, putRequest)
	if putResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on PUT without write scope, got: %d", putResponseRecorder.Code)
	}

	// 3. GET with invalid key fails with 403
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/config", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer sec_live_invalid_key_value")
	invalidKeyResponseRecorder := httptest.NewRecorder()
	configManager.HandleGetConfig(invalidKeyResponseRecorder, invalidKeyRequest)
	if invalidKeyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on invalid key, got: %d", invalidKeyResponseRecorder.Code)
	}
}

func TestAuthConfigManagerBrokenPoolIntegration(t *testing.T) {
	brokenDB := createBrokenPool(t)
	if brokenDB == nil {
		t.Skip("skipping broken pool test")
		return
	}

	cryptoKeyManager, _ := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	configManager := NewConfigManager(brokenDB, cryptoKeyManager)
	ctx := context.Background()

	// Load with broken pool returns error
	if err := configManager.Load(ctx); err == nil {
		t.Fatal("expected error loading with broken pool")
	}

	// Save with broken pool returns error
	if err := configManager.Save(ctx, DefaultConfig()); err == nil {
		t.Fatal("expected error saving with broken pool")
	}

	// HandlePutConfig with broken pool returns 500
	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/auth/config", strings.NewReader(`{}`))
	putResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(putResponseRecorder, putRequest)
	if putResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on broken pool, got: %d", putResponseRecorder.Code)
	}
}
