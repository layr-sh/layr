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
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/test", nil)
	WriteErrorResponse(recorder, request, http.StatusNotFound, "Resource not found", "LAYR_CORE_002")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %s", contentType)
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if response.Type != "https://layr.sh/errors/layr-core-002" {
		t.Errorf("unexpected Type: %s", response.Type)
	}
	if response.Title != "Not Found" {
		t.Errorf("unexpected Title: %s", response.Title)
	}
	if response.Status != http.StatusNotFound {
		t.Errorf("unexpected Status: %d", response.Status)
	}
	if response.Detail != "Resource not found" {
		t.Errorf("unexpected Detail: %s", response.Detail)
	}
	if response.Instance != "/api/v1/test" {
		t.Errorf("unexpected Instance: %s", response.Instance)
	}
	if response.ErrorCode != "LAYR_CORE_002" {
		t.Errorf("unexpected ErrorCode: %s", response.ErrorCode)
	}
	if response.Error != "layr_core_002" {
		t.Errorf("unexpected Error: %s", response.Error)
	}
	if response.ErrorDescription != "Resource not found" {
		t.Errorf("unexpected ErrorDescription: %s", response.ErrorDescription)
	}
	if response.Timestamp == "" {
		t.Errorf("expected non-empty timestamp")
	}

	// 2. Test WriteErrorResponse with non-standard status code (hitting title == "" fallback)
	recorderUnknown := httptest.NewRecorder()
	WriteErrorResponse(recorderUnknown, request, 999, "Custom fallback", "")
	if recorderUnknown.Code != 999 {
		t.Fatalf("expected status 999, got %d", recorderUnknown.Code)
	}

	var unknownResponse ErrorResponse
	if err := json.Unmarshal(recorderUnknown.Body.Bytes(), &unknownResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if unknownResponse.Title != "API Error" {
		t.Errorf("unexpected fallback title: %s", unknownResponse.Title)
	}
	if unknownResponse.Type != "https://layr.sh/errors/api-error" {
		t.Errorf("unexpected fallback type: %s", unknownResponse.Type)
	}

	// 3. Test WriteErrorResponseProblem with nil request (hitting request == nil branch)
	recorderNilRequest := httptest.NewRecorder()
	WriteErrorResponseProblem(recorderNilRequest, nil, http.StatusBadRequest, "Invalid Input", "Bad syntax", "")
	var nilRequestResponse ErrorResponse
	if err := json.Unmarshal(recorderNilRequest.Body.Bytes(), &nilRequestResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if nilRequestResponse.Instance != "" {
		t.Errorf("expected empty instance for nil request, got %s", nilRequestResponse.Instance)
	}
	if nilRequestResponse.Type != "https://layr.sh/errors/invalid-input" {
		t.Errorf("unexpected type: %s", nilRequestResponse.Type)
	}

	// 4. Test WriteErrorResponseProblem with custom non-empty errorCode
	recorderWithCode := httptest.NewRecorder()
	WriteErrorResponseProblem(recorderWithCode, request, http.StatusForbidden, "Forbidden Action", "You cannot access this resource", "AUTH_FORBIDDEN_001")
	var withCodeResponse ErrorResponse
	if err := json.Unmarshal(recorderWithCode.Body.Bytes(), &withCodeResponse); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if withCodeResponse.Type != "https://layr.sh/errors/auth-forbidden-001" {
		t.Errorf("unexpected type: %s", withCodeResponse.Type)
	}
}
