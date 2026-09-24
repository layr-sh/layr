package data

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerConfigGetAndUpdateUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// 1. handleGetConfig success
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetConfig(getResponseRecorder, getRequest)
	if getResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResponseRecorder.Code)
	}

	// 2. handleUpdateConfig invalid JSON
	invalidJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader([]byte("{invalid")))
	invalidJSONResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(invalidJSONResponseRecorder, invalidJSONRequest)
	if invalidJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid JSON, got %d", invalidJSONResponseRecorder.Code)
	}

	// 3. handleUpdateConfig invalid config -> 400
	invalidConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader([]byte(`{"rest":{"max_limit":0}}`)))
	invalidConfigResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(invalidConfigResponseRecorder, invalidConfigRequest)
	if invalidConfigResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid config, got %d", invalidConfigResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerConfigScopeForbiddenUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Forbidden Get
	forbiddenGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/config", nil)
	forbiddenGetRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetConfig(forbiddenGetResponseRecorder, forbiddenGetRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthorized GET config, got %d", forbiddenGetResponseRecorder.Code)
	}

	// Forbidden Update
	forbiddenUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/data/config", bytes.NewReader([]byte(`{}`)))
	forbiddenUpdateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenUpdateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateConfig(forbiddenUpdateResponseRecorder, forbiddenUpdateRequest)
	if forbiddenUpdateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthorized PUT config, got %d", forbiddenUpdateResponseRecorder.Code)
	}

	// Forbidden Flush
	forbiddenFlushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	forbiddenFlushRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenFlushResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleFlushCache(forbiddenFlushResponseRecorder, forbiddenFlushRequest)
	if forbiddenFlushResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthorized POST cache flush, got %d", forbiddenFlushResponseRecorder.Code)
	}

	// Forbidden Invalidate
	forbiddenInvalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", nil)
	forbiddenInvalidateRequest.Header.Set("Authorization", "Bearer invalid_key")
	forbiddenInvalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleInvalidateCache(forbiddenInvalidateResponseRecorder, forbiddenInvalidateRequest)
	if forbiddenInvalidateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthorized POST cache invalidate, got %d", forbiddenInvalidateResponseRecorder.Code)
	}
}

func TestDataControlPlaneHandlerConfigCacheControlUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler

	// Flush Cache wrong method
	invalidMethodFlushRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/cache/flush", nil)
	invalidMethodFlushResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleFlushCache(invalidMethodFlushResponseRecorder, invalidMethodFlushRequest)
	if invalidMethodFlushResponseRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET /cache/flush, got %d", invalidMethodFlushResponseRecorder.Code)
	}

	// Flush Cache success
	validFlushRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/flush", nil)
	validFlushResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleFlushCache(validFlushResponseRecorder, validFlushRequest)
	if validFlushResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST /cache/flush, got %d", validFlushResponseRecorder.Code)
	}

	// Invalidate Cache wrong method
	invalidMethodInvalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/cache/invalidate", nil)
	invalidMethodInvalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleInvalidateCache(invalidMethodInvalidateResponseRecorder, invalidMethodInvalidateRequest)
	if invalidMethodInvalidateResponseRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET /cache/invalidate, got %d", invalidMethodInvalidateResponseRecorder.Code)
	}

	// Invalidate Cache success
	validInvalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"catalog":true}`)))
	validInvalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleInvalidateCache(validInvalidateResponseRecorder, validInvalidateRequest)
	if validInvalidateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST /cache/invalidate, got %d", validInvalidateResponseRecorder.Code)
	}

	// Invalidate Cache with pattern
	patternInvalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/cache/invalidate", bytes.NewReader([]byte(`{"pattern":"items:*"}`)))
	patternInvalidateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleInvalidateCache(patternInvalidateResponseRecorder, patternInvalidateRequest)
	if patternInvalidateResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST /cache/invalidate with pattern, got %d", patternInvalidateResponseRecorder.Code)
	}
}
