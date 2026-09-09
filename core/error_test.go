package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreErrorResponseSerializationAndHelpersUnit(t *testing.T) {
	// 1. Test WriteErrorResponse with standard status code and error code
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/test", nil)
	WriteErrorResponse(responseRecorder, request, http.StatusNotFound, "Resource not found", "LAYR_CORE_002")

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %s", contentType)
	}

	var errorResponse ErrorResponse
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if errorResponse.Type != "https://layr.sh/errors/layr-core-002" {
		t.Errorf("unexpected Type: %s", errorResponse.Type)
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
	if errorResponse.ErrorCode != "LAYR_CORE_002" {
		t.Errorf("unexpected ErrorCode: %s", errorResponse.ErrorCode)
	}
	if errorResponse.Error != "layr_core_002" {
		t.Errorf("unexpected Error: %s", errorResponse.Error)
	}
	if errorResponse.ErrorDescription != "Resource not found" {
		t.Errorf("unexpected ErrorDescription: %s", errorResponse.ErrorDescription)
	}
	if errorResponse.Timestamp == "" {
		t.Errorf("expected non-empty timestamp")
	}

	// 2. Test WriteErrorResponse with non-standard status code (hitting title == "" fallback)
	unknownResponseRecorder := httptest.NewRecorder()
	WriteErrorResponse(unknownResponseRecorder, request, 999, "Custom fallback", "")
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
	if unknownErrorResponse.Type != "https://layr.sh/errors/api-error" {
		t.Errorf("unexpected fallback type: %s", unknownErrorResponse.Type)
	}

	// 3. Test WriteErrorResponseProblem with nil request (hitting request == nil branch)
	nilRequestResponseRecorder := httptest.NewRecorder()
	WriteErrorResponseProblem(nilRequestResponseRecorder, nil, http.StatusBadRequest, "Invalid Input", "Bad syntax", "")
	var nilRequestErrorResponse ErrorResponse
	if err := json.Unmarshal(nilRequestResponseRecorder.Body.Bytes(), &nilRequestErrorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if nilRequestErrorResponse.Instance != "" {
		t.Errorf("expected empty instance for nil request, got %s", nilRequestErrorResponse.Instance)
	}
	if nilRequestErrorResponse.Type != "https://layr.sh/errors/invalid-input" {
		t.Errorf("unexpected type: %s", nilRequestErrorResponse.Type)
	}

	// 4. Test WriteErrorResponseProblem with custom non-empty errorCode
	withCodeResponseRecorder := httptest.NewRecorder()
	WriteErrorResponseProblem(withCodeResponseRecorder, request, http.StatusForbidden, "Forbidden Action", "You cannot access this resource", "AUTH_FORBIDDEN_001")
	var withCodeErrorResponse ErrorResponse
	if err := json.Unmarshal(withCodeResponseRecorder.Body.Bytes(), &withCodeErrorResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if withCodeErrorResponse.Type != "https://layr.sh/errors/auth-forbidden-001" {
		t.Errorf("unexpected type: %s", withCodeErrorResponse.Type)
	}
}
