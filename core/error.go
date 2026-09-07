package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ErrorResponse represents an RFC 7807 problem details and RFC 6749 OAuth response model.
type ErrorResponse struct {
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	Type             string `json:"type"`
	Title            string `json:"title"`
	Status           int    `json:"status"`
	Detail           string `json:"detail"`
	Instance         string `json:"instance,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	Timestamp        string `json:"timestamp"`
}

// WriteErrorResponse writes a standardized RFC 7807 error response using the standard HTTP status text as title.
func WriteErrorResponse(responseWriter http.ResponseWriter, request *http.Request, status int, detail string, errorCode string) {
	title := http.StatusText(status)
	if title == "" {
		title = "API Error"
	}
	WriteErrorResponseProblem(responseWriter, request, status, title, detail, errorCode)
}

// WriteErrorResponseProblem writes a standardized RFC 7807 problem response with an explicit custom title.
func WriteErrorResponseProblem(responseWriter http.ResponseWriter, request *http.Request, status int, title string, detail string, errorCode string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)

	slug := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	if errorCode != "" {
		slug = strings.ToLower(strings.ReplaceAll(errorCode, "_", "-"))
	}
	typeURL := "https://layr.sh/errors/" + slug

	var instance string
	if request != nil && request.URL != nil {
		instance = request.URL.Path
	}

	oauthError := slug
	if errorCode != "" {
		oauthError = strings.ToLower(errorCode)
	}

	errResponse := ErrorResponse{
		Error:            oauthError,
		ErrorDescription: detail,
		Type:             typeURL,
		Title:            title,
		Status:           status,
		Detail:           detail,
		Instance:         instance,
		ErrorCode:        errorCode,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(responseWriter).Encode(errResponse)
}
