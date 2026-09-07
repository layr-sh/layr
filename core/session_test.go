package core

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCoreSessionCookieAndRequestHelpersUnit(t *testing.T) {
	// 1. IsSecureRequest tests
	if IsSecureRequest(nil) {
		t.Fatal("expected nil request to be insecure")
	}

	plainRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/", nil)
	if IsSecureRequest(plainRequest) {
		t.Fatal("expected plain http to be insecure")
	}

	tlsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://localhost/", nil)
	tlsRequest.TLS = &tls.ConnectionState{}
	if !IsSecureRequest(tlsRequest) {
		t.Fatal("expected TLS request to be secure")
	}

	forwardedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/", nil)
	forwardedRequest.Header.Set("X-Forwarded-Proto", "https")
	if !IsSecureRequest(forwardedRequest) {
		t.Fatal("expected X-Forwarded-Proto https to be secure")
	}

	forwardedUpperRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/", nil)
	forwardedUpperRequest.Header.Set("X-Forwarded-Proto", "HTTPS")
	if !IsSecureRequest(forwardedUpperRequest) {
		t.Fatal("expected X-Forwarded-Proto HTTPS to be secure")
	}

	forwardedInsecureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/", nil)
	forwardedInsecureRequest.Header.Set("X-Forwarded-Proto", "http")
	if IsSecureRequest(forwardedInsecureRequest) {
		t.Fatal("expected X-Forwarded-Proto http to be insecure")
	}

	// 2. ExtractRequestClientIP tests
	if clientIP := ExtractRequestClientIP(nil); clientIP != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1 for nil, got %s", clientIP)
	}

	forwardedForRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	forwardedForRequest.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18")
	if clientIP := ExtractRequestClientIP(forwardedForRequest); clientIP != "203.0.113.1" {
		t.Fatalf("expected 203.0.113.1, got %s", clientIP)
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

	ipv6PortRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ipv6PortRequest.RemoteAddr = "[::1]:54321"
	if clientIP := ExtractRequestClientIP(ipv6PortRequest); clientIP != "::1" {
		t.Fatalf("expected ::1, got %s", clientIP)
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

	// 3. SetSessionCookie and ClearSessionCookie tests
	cookieRecorder := httptest.NewRecorder()
	expirationTime := time.Now().Add(time.Hour)

	// Set secure cookie
	SetSessionCookie(cookieRecorder, "__Host-test", "test", "token_value_secure", expirationTime, true)
	// Set plain cookie
	SetSessionCookie(cookieRecorder, "__Host-test", "test", "token_value_plain", expirationTime, false)

	// Clear cookies
	ClearSessionCookie(cookieRecorder, "__Host-test", "test", true)
	ClearSessionCookie(cookieRecorder, "__Host-test", "test", false)

	// 4. ExtractRequestSessionToken tests
	if token := ExtractRequestSessionToken(nil, "__Host-test", "test"); token != "" {
		t.Fatalf("expected empty token for nil request, got %s", token)
	}

	bearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerRequest.Header.Set("Authorization", "Bearer bearer_token_123")
	if token := ExtractRequestSessionToken(bearerRequest, "__Host-test", "test"); token != "bearer_token_123" {
		t.Fatalf("expected bearer_token_123, got %s", token)
	}

	bearerLowerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerLowerRequest.Header.Set("Authorization", "bearer bearer_token_lower")
	if token := ExtractRequestSessionToken(bearerLowerRequest, "__Host-test", "test"); token != "bearer_token_lower" {
		t.Fatalf("expected bearer_token_lower, got %s", token)
	}

	secureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	secureCookieRequest.AddCookie(&http.Cookie{Name: "__Host-test", Value: "secure_cookie_value"})
	if token := ExtractRequestSessionToken(secureCookieRequest, "__Host-test", "test"); token != "secure_cookie_value" {
		t.Fatalf("expected secure_cookie_value, got %s", token)
	}

	plainCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	plainCookieRequest.AddCookie(&http.Cookie{Name: "test", Value: "plain_cookie_value"})
	if token := ExtractRequestSessionToken(plainCookieRequest, "__Host-test", "test"); token != "plain_cookie_value" {
		t.Fatalf("expected plain_cookie_value, got %s", token)
	}

	emptyTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if token := ExtractRequestSessionToken(emptyTokenRequest, "__Host-test", "test"); token != "" {
		t.Fatalf("expected empty token, got %s", token)
	}
}
