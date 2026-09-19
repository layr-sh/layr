package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreErrorResponseSerializationAndHelpersUnit(t *testing.T) {
	// 1. Test WriteErrorResponse with standard status code and debugLog
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/test", nil)
	WriteErrorResponse(responseRecorder, request, http.StatusNotFound, "Resource not found", "item-123 not found in db")

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %s", contentType)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("failed to decode JSON into map: %v", err)
	}
	if _, exists := rawMap["type"]; exists {
		t.Errorf("expected no 'type' property in RFC 9457 problem response, but found: %v", rawMap["type"])
	}
	if _, exists := rawMap["error"]; exists {
		t.Errorf("expected no 'error' property in RFC 9457 problem response, but found: %v", rawMap["error"])
	}
	if _, exists := rawMap["error_description"]; exists {
		t.Errorf("expected no 'error_description' property in RFC 9457 problem response, but found: %v", rawMap["error_description"])
	}
	if _, exists := rawMap["error_code"]; exists {
		t.Errorf("expected no 'error_code' property in RFC 9457 problem response, but found: %v", rawMap["error_code"])
	}

	var errorResponse ErrorResponse
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if errorResponse.Title != "Not Found" {
		t.Errorf("unexpected Title: %s", errorResponse.Title)
	}
	if errorResponse.Status != http.StatusNotFound {
		t.Errorf("unexpected Status: %d", errorResponse.Status)
	}
	if errorResponse.Detail != "Resource not found" {
		t.Errorf("unexpected Detail: %s", errorResponse.Detail)
	}
	if errorResponse.Instance != "/api/v1/test" {
		t.Errorf("unexpected Instance: %s", errorResponse.Instance)
	}
	if errorResponse.Timestamp == "" {
		t.Errorf("expected non-empty timestamp")
	}

	// 2. Test WriteErrorResponse with non-standard status code without debugLog (hitting title == "" fallback)
	unknownResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(unknownResponseRecorder, request, 999, "Custom fallback")
	if unknownResponseRecorder.Code != 999 {
		t.Fatalf("expected status 999, got %d", unknownResponseRecorder.Code)
	}

	var unknownErrorResponse ErrorResponse
	if err := json.Unmarshal(unknownResponseRecorder.Body.Bytes(), &unknownErrorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if unknownErrorResponse.Title != "API Error" {
		t.Errorf("unexpected fallback title: %s", unknownErrorResponse.Title)
	}

	// 3. Test WriteErrorResponse with nil request (hitting request == nil branch) without debugLog
	nilRequestResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(nilRequestResponseRecorder, nil, http.StatusBadRequest, "Invalid Input")
	var nilRequestErrorResponse ErrorResponse
	if err := json.Unmarshal(nilRequestResponseRecorder.Body.Bytes(), &nilRequestErrorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if nilRequestErrorResponse.Instance != "" {
		t.Errorf("expected empty instance for nil request, got %s", nilRequestErrorResponse.Instance)
	}

	// 4. Test WriteErrorResponse with status 403 Forbidden and debugLog
	withDebugResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(withDebugResponseRecorder, request, http.StatusForbidden, "You cannot access this resource", "user missing admin scope")
	if withDebugResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", withDebugResponseRecorder.Code)
	}

	// 5. Test WriteErrorResponse with 500 Internal Server Error (status >= 500 branch)
	serverErrorResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(serverErrorResponseRecorder, request, http.StatusInternalServerError, "An unexpected error occurred", "pq: connection refused")
	if serverErrorResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", serverErrorResponseRecorder.Code)
	}

	// 6. Test WriteErrorResponse with status < 400 (else branch)
	lowStatusResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(lowStatusResponseRecorder, request, http.StatusOK, "Everything normal", "debug info for 200")
	if lowStatusResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", lowStatusResponseRecorder.Code)
	}
}

func TestCoreOAuthErrorResponseSerializationAndHelpersUnit(t *testing.T) {
	// 1. Test WriteOAuthErrorResponse standard writer
	responseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(responseRecorder, http.StatusBadRequest, "invalid_request", "Missing client_id parameter")

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", responseRecorder.Code)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected application/json Content-Type, got %s", contentType)
	}
	if cacheControl := responseRecorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("expected Cache-Control no-store, got %s", cacheControl)
	}
	if pragma := responseRecorder.Header().Get("Pragma"); pragma != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %s", pragma)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("failed to decode JSON into map: %v", err)
	}
	if _, exists := rawMap["error_uri"]; exists {
		t.Errorf("expected no 'error_uri' property in OAuth error response, but found: %v", rawMap["error_uri"])
	}

	var oauthErrorResponse OAuthErrorResponse
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &oauthErrorResponse); err != nil {
		t.Fatalf("failed to unmarshal OAuthErrorResponse: %v", err)
	}
	if oauthErrorResponse.Error != "invalid_request" {
		t.Errorf("expected error invalid_request, got %s", oauthErrorResponse.Error)
	}
	if oauthErrorResponse.ErrorDescription != "Missing client_id parameter" {
		t.Errorf("expected error_description to match, got %s", oauthErrorResponse.ErrorDescription)
	}

	// 2. Test WriteOAuthErrorResponse with 401 Unauthorized (invalid_client -> Basic realm="OAuth")
	unauthorizedResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(unauthorizedResponseRecorder, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
	if unauthorizedResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", unauthorizedResponseRecorder.Code)
	}
	if wwwAuth := unauthorizedResponseRecorder.Header().Get("WWW-Authenticate"); wwwAuth != `Basic realm="OAuth"` {
		t.Errorf("expected Basic realm=\"OAuth\", got %s", wwwAuth)
	}
	var unauthorizedOAuthErrorResponse OAuthErrorResponse
	if err := json.Unmarshal(unauthorizedResponseRecorder.Body.Bytes(), &unauthorizedOAuthErrorResponse); err != nil {
		t.Fatalf("failed to unmarshal OAuthErrorResponse: %v", err)
	}
	if unauthorizedOAuthErrorResponse.Error != "invalid_client" {
		t.Errorf("expected error invalid_client, got %s", unauthorizedOAuthErrorResponse.Error)
	}

	// 2b. Test WriteOAuthErrorResponse with 401 Unauthorized (invalid_token with description -> Bearer with description)
	bearerDescResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(bearerDescResponseRecorder, http.StatusUnauthorized, "invalid_token", "Token expired")
	if wwwAuth := bearerDescResponseRecorder.Header().Get("WWW-Authenticate"); wwwAuth != `Bearer error="invalid_token", error_description="Token expired"` {
		t.Errorf("expected Bearer header with description, got %s", wwwAuth)
	}

	// 2c. Test WriteOAuthErrorResponse with 401 Unauthorized (invalid_token without description)
	bearerNoDescResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(bearerNoDescResponseRecorder, http.StatusUnauthorized, "invalid_token", "")
	if wwwAuth := bearerNoDescResponseRecorder.Header().Get("WWW-Authenticate"); wwwAuth != `Bearer error="invalid_token"` {
		t.Errorf("expected Bearer header without description, got %s", wwwAuth)
	}

	// 2d. Test WriteOAuthErrorResponse with 401 Unauthorized and pre-set WWW-Authenticate
	preSetResponseRecorder := httptest.NewRecorder()
	preSetResponseRecorder.Header().Set("WWW-Authenticate", "CustomScheme")
	WriteOAuthErrorResponse(preSetResponseRecorder, http.StatusUnauthorized, "invalid_token", "Custom")
	if wwwAuth := preSetResponseRecorder.Header().Get("WWW-Authenticate"); wwwAuth != "CustomScheme" {
		t.Errorf("expected pre-set WWW-Authenticate CustomScheme, got %s", wwwAuth)
	}

	// 3. Test logging branches with debugLog (status >= 500 and status < 400)
	serverErrorResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(serverErrorResponseRecorder, http.StatusInternalServerError, "server_error", "Unexpected error", "sql connection failure")
	if serverErrorResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", serverErrorResponseRecorder.Code)
	}

	warnStatusResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(warnStatusResponseRecorder, http.StatusBadRequest, "invalid_request", "Bad client input", "warn trace")
	if warnStatusResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", warnStatusResponseRecorder.Code)
	}

	lowStatusResponseRecorder := httptest.NewRecorder()
	WriteOAuthErrorResponse(lowStatusResponseRecorder, http.StatusOK, "ok_notice", "Notice", "info trace")
	if lowStatusResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", lowStatusResponseRecorder.Code)
	}
}
