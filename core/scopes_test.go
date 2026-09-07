package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreScopesHasScopeUnit(t *testing.T) {
	tests := []struct {
		name     string
		granted  []string
		required string
		expected bool
	}{
		{
			name:     "Empty granted",
			granted:  []string{},
			required: "data:schema.read",
			expected: false,
		},
		{
			name:     "Empty required",
			granted:  []string{"data:schema.read"},
			required: "",
			expected: true,
		},
		{
			name:     "Root wildcard * matches everything",
			granted:  []string{"*"},
			required: "data:schema.read",
			expected: true,
		},
		{
			name:     "Exact match",
			granted:  []string{"data:schema.read"},
			required: "data:schema.read",
			expected: true,
		},
		{
			name:     "Mismatch service",
			granted:  []string{"auth:user.read"},
			required: "data:schema.read",
			expected: false,
		},
		{
			name:     "Mismatch resource",
			granted:  []string{"data:query.read"},
			required: "data:schema.read",
			expected: false,
		},
		{
			name:     "Mismatch action (read does not grant write)",
			granted:  []string{"data:schema.read"},
			required: "data:schema.write",
			expected: false,
		},
		{
			name:     "Write implies Read",
			granted:  []string{"data:schema.write"},
			required: "data:schema.read",
			expected: true,
		},
		{
			name:     "Service wildcard data:* matches data:schema.read",
			granted:  []string{"data:*"},
			required: "data:schema.read",
			expected: true,
		},
		{
			name:     "Service wildcard data:* matches data:config.write",
			granted:  []string{"data:*"},
			required: "data:config.write",
			expected: true,
		},
		{
			name:     "Service wildcard data:* does not match auth:user.read",
			granted:  []string{"data:*"},
			required: "auth:user.read",
			expected: false,
		},
		{
			name:     "Resource wildcard data:schema matches data:schema.write",
			granted:  []string{"data:schema"},
			required: "data:schema.write",
			expected: true,
		},
		{
			name:     "Global action wildcard *:*.read matches data:schema.read",
			granted:  []string{"*:*.read"},
			required: "data:schema.read",
			expected: true,
		},
		{
			name:     "Global action wildcard *:*.read does not match data:schema.write",
			granted:  []string{"*:*.read"},
			required: "data:schema.write",
			expected: false,
		},
		{
			name:     "Multiple granted scopes first match",
			granted:  []string{"auth:user.read", "data:query.write", "storage:bucket.read"},
			required: "data:query.read",
			expected: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := HasScope(testCase.granted, testCase.required)
			if result != testCase.expected {
				t.Fatalf("expected %v, got %v for granted=%v, required=%v", testCase.expected, result, testCase.granted, testCase.required)
			}
		})
	}
}

func TestCoreScopesParseScopeUnit(t *testing.T) {
	parsedService, parsedResource, parsedAction := parseScope("data")
	if parsedService != "data" || parsedResource != "*" || parsedAction != "*" {
		t.Fatalf("unexpected parse: %s, %s, %s", parsedService, parsedResource, parsedAction)
	}

	parsedService, parsedResource, parsedAction = parseScope("*")
	if parsedService != "*" || parsedResource != "*" || parsedAction != "*" {
		t.Fatalf("unexpected parse: %s, %s, %s", parsedService, parsedResource, parsedAction)
	}

	parsedService, parsedResource, parsedAction = parseScope("")
	if parsedService != "*" || parsedResource != "*" || parsedAction != "*" {
		t.Fatalf("unexpected parse: %s, %s, %s", parsedService, parsedResource, parsedAction)
	}
}

func TestCoreScopesExtractRequestClientIPUnit(t *testing.T) {
	if clientIP := ExtractRequestClientIP(nil); clientIP != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1 for nil, got %s", clientIP)
	}

	forwardedForRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	forwardedForRequest.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	if clientIP := ExtractRequestClientIP(forwardedForRequest); clientIP != "203.0.113.195" {
		t.Fatalf("expected 203.0.113.195, got %s", clientIP)
	}

	realIPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	realIPRequest.Header.Set("X-Real-IP", "198.51.100.1")
	if clientIP := ExtractRequestClientIP(realIPRequest); clientIP != "198.51.100.1" {
		t.Fatalf("expected 198.51.100.1, got %s", clientIP)
	}

	ipv4PortRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ipv4PortRequest.RemoteAddr = "192.0.2.1:12345"
	if clientIP := ExtractRequestClientIP(ipv4PortRequest); clientIP != "192.0.2.1" {
		t.Fatalf("expected 192.0.2.1, got %s", clientIP)
	}

	bareIPv6Request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bareIPv6Request.RemoteAddr = "[::1]"
	if clientIP := ExtractRequestClientIP(bareIPv6Request); clientIP != "::1" {
		t.Fatalf("expected ::1, got %s", clientIP)
	}

	emptyRemoteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	emptyRemoteRequest.RemoteAddr = ""
	if clientIP := ExtractRequestClientIP(emptyRemoteRequest); clientIP != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1 for empty remoteAddr, got %s", clientIP)
	}
}
