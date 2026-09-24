package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrorResponse represents an HTTP problem details response model based on RFC 9457 without a type URI.
type ErrorResponse struct {
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Instance  string `json:"instance,omitempty"`
	Timestamp string `json:"timestamp"`
}

// OAuthErrorResponse represents a standard OAuth 2.0 error response model based on RFC 6749 Section 5.2.
type OAuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// WriteErrorResponse writes a standardized RFC 9457 error response (without type URI).
// debugLog is an optional parameter; when provided, its content is logged to the server logs.
func WriteErrorResponse(responseWriter http.ResponseWriter, request *http.Request, status int, detail string, debugLog ...string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)

	title := http.StatusText(status)
	if title == "" {
		title = "API Error"
	}

	var instance string
	if request != nil && request.URL != nil {
		instance = request.URL.Path
	}

	errorResponse := ErrorResponse{
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  instance,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	var debugMessage string
	if len(debugLog) > 0 {
		debugMessage = debugLog[0]
	}

	if debugMessage != "" {
		if status >= http.StatusInternalServerError {
			log.Error(debugMessage)
		} else if status >= http.StatusBadRequest {
			log.Warn(debugMessage)
		} else {
			log.Debug(debugMessage)
		}
	} else if status >= http.StatusInternalServerError {
		log.Errorf("internal server error (%d): %s", status, detail)
	}

	_ = json.NewEncoder(responseWriter).Encode(errorResponse)
}

// WriteOAuthErrorResponse writes a standard RFC 6749 OAuth 2.0 error response.
// debugLog is an optional parameter; when provided, its content is logged to the server logs.
func WriteOAuthErrorResponse(responseWriter http.ResponseWriter, status int, oauthError string, errorDescription string, debugLog ...string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.Header().Set("Pragma", "no-cache")
	if status == http.StatusUnauthorized && responseWriter.Header().Get("WWW-Authenticate") == "" {
		if oauthError == "invalid_client" {
			responseWriter.Header().Set("WWW-Authenticate", `Basic realm="OAuth"`)
		} else if errorDescription != "" {
			responseWriter.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="%s", error_description="%s"`, oauthError, errorDescription))
		} else {
			responseWriter.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="%s"`, oauthError))
		}
	}
	responseWriter.WriteHeader(status)

	oauthErrorResponse := OAuthErrorResponse{
		Error:            oauthError,
		ErrorDescription: errorDescription,
	}

	var debugMessage string
	if len(debugLog) > 0 {
		debugMessage = debugLog[0]
	}

	if debugMessage != "" {
		if status >= http.StatusInternalServerError {
			log.Error(debugMessage)
		} else if status >= http.StatusBadRequest {
			log.Warn(debugMessage)
		} else {
			log.Debug(debugMessage)
		}
	} else if status >= http.StatusInternalServerError {
		log.Errorf("internal oauth error (%d): %s", status, errorDescription)
	}

	_ = json.NewEncoder(responseWriter).Encode(oauthErrorResponse)
}
