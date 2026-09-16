package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataControlPlaneHandlerConfigLifecycleIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	defer cleanup()

	ctx := context.Background()
	service := NewService(db)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// 1. Get Config
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/data/config", nil)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetConfig(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET config, got %d", getResponseRecorder.Code)
	}

	var retrievedConfig Config
	if err := json.NewDecoder(getResponseRecorder.Body).Decode(&retrievedConfig); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}

	// 2. Update Config
	retrievedConfig.REST.MaxLimit = 500
	updatePayload, err := json.Marshal(retrievedConfig)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/data/config", bytes.NewReader(updatePayload))
	updateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUpdateConfig(updateResponseRecorder, updateRequest)
	if updateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT config, got %d", updateResponseRecorder.Code)
	}

	// 3. Flush Cache
	flushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/flush", nil)
	flushResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleFlushCache(flushResponseRecorder, flushRequest)
	if flushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on cache flush, got %d", flushResponseRecorder.Code)
	}

	// 4. Invalidate Cache
	invalidatePayload := `{"schema":"public","table":"users"}`
	invalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/cache/invalidate", bytes.NewReader([]byte(invalidatePayload)))
	invalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleInvalidateCache(invalidateResponseRecorder, invalidateRequest)
	if invalidateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on cache invalidate, got %d", invalidateResponseRecorder.Code)
	}
}
