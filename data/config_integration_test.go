package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataConfigPostgresIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db)

	// 1. Initial Load when key 'runtime' does not exist -> seeds DefaultConfig
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed initial Load: %v", err)
	}
	initialConfig := configManager.Get()
	if !initialConfig.REST.Enabled || initialConfig.REST.MaxLimit != 1000 {
		t.Fatalf("unexpected initial settings: %+v", initialConfig)
	}

	// 2. Set with custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.REST.MaxLimit = 500
	customConfig.REST.DefaultLimit = 25
	customConfig.GraphQL.MaxDepth = 12
	customConfig.Realtime.HeartbeatIntervalMS = 15000

	if err := configManager.Set(ctx, customConfig); err != nil {
		t.Fatalf("failed to set custom config: %v", err)
	}

	freshConfigManager := NewConfigManager(db)
	if err := freshConfigManager.Load(ctx); err != nil {
		t.Fatalf("failed fresh Load: %v", err)
	}
	loadedConfig := freshConfigManager.Get()
	if loadedConfig.REST.MaxLimit != 500 || loadedConfig.REST.DefaultLimit != 25 || loadedConfig.GraphQL.MaxDepth != 12 {
		t.Fatalf("mismatched loaded settings: %+v", loadedConfig)
	}

	// 3. Set with zero/negative fields triggers clamping fallbacks in Set
	zeroConfig := Config{}
	if err := configManager.Set(ctx, zeroConfig); err != nil {
		t.Fatalf("failed to set zero config: %v", err)
	}
	clampedConfig := configManager.Get()
	if clampedConfig.REST.MaxLimit != 1000 || clampedConfig.REST.DefaultLimit != 50 || clampedConfig.GraphQL.MaxDepth != 8 {
		t.Fatalf("expected clamped values on zero config, got: %+v", clampedConfig)
	}

	// 4. HandlePutConfig HTTP endpoint with valid JSON
	putPayload, _ := json.Marshal(customConfig)
	putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/data/config", bytes.NewReader(putPayload))
	putResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(putResponseRecorder, putRequest)

	if putResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on HandlePutConfig, got %d", putResponseRecorder.Code)
	}

	// 5. Corrupt JSON in database causes Load to return error
	const corruptSQLStatement = `UPDATE data.config SET value = '"invalid json string not object"'::jsonb WHERE key = 'runtime'`
	if _, execErr := db.Exec(ctx, corruptSQLStatement); execErr != nil {
		t.Fatalf("failed to corrupt config JSON: %v", execErr)
	}
	if err := configManager.Load(ctx); err == nil {
		t.Fatal("expected Load error on invalid JSON structure in DB")
	}

	// 6. DB execution error on Set (e.g. canceled context)
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := configManager.Set(canceledCtx, customConfig); err == nil {
		t.Fatal("expected error on Set with canceled context")
	}

	// 7. HandlePutConfig with closed/failing DB causes 500 Internal Server Error
	closedContextRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPut, "/api/v1/_/data/config", bytes.NewReader(putPayload))
	closedContextResponseRecorder := httptest.NewRecorder()
	configManager.HandlePutConfig(closedContextResponseRecorder, closedContextRequest)
	if closedContextResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing DB in HandlePutConfig, got %d", closedContextResponseRecorder.Code)
	}
}
