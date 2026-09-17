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

	// Untrusted proxy test: X-Forwarded-For is ignored, RemoteAddr is used
	untrustedConfig := DefaultConfig()
	untrustedConfig.Server.TrustProxyHeaders = false
	SetLoadedConfig(untrustedConfig)
	spoofedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	spoofedRequest.Header.Set("X-Forwarded-For", "203.0.113.1")
	spoofedRequest.RemoteAddr = "192.0.2.100:12345"
	if clientIP := ExtractRequestClientIP(spoofedRequest); clientIP != "192.0.2.100" {
		t.Fatalf("expected remote addr 192.0.2.100 when proxy headers untrusted, got %s", clientIP)
	}
	UnloadConfig()

	// 3. SetSessionCookie and ClearSessionCookie tests
	expirationTime := time.Now().Add(time.Hour)
	secureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	secureRequest.TLS = &tls.ConnectionState{}
	insecureRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)

	secureResponseRecorder := httptest.NewRecorder()
	SetSessionCookie(secureResponseRecorder, secureRequest, "token_value_secure", expirationTime)
	secCookies := secureResponseRecorder.Result().Cookies()
	var foundSecure bool
	for _, cookie := range secCookies {
		if cookie.Name == SessionCookieNameSecure && cookie.Value == "token_value_secure" && cookie.Secure {
			foundSecure = true
		}
	}
	if !foundSecure {
		t.Fatal("expected secure session cookie to be set")
	}

	insecureResponseRecorder := httptest.NewRecorder()
	SetSessionCookie(insecureResponseRecorder, insecureRequest, "token_value_plain", expirationTime)
	insecCookies := insecureResponseRecorder.Result().Cookies()
	var foundInsecure bool
	for _, cookie := range insecCookies {
		if cookie.Name == SessionCookieNameInsecure && cookie.Value == "token_value_plain" && !cookie.Secure {
			foundInsecure = true
		}
	}
	if !foundInsecure {
		t.Fatal("expected insecure session cookie to be set")
	}

	clearResponseRecorder := httptest.NewRecorder()
	ClearSessionCookie(clearResponseRecorder, secureRequest)
	ClearSessionCookie(clearResponseRecorder, insecureRequest)

	// 4. ExtractRequestSessionToken tests
	if token := ExtractRequestSessionToken(nil); token != "" {
		t.Fatalf("expected empty token for nil request, got %s", token)
	}

	bearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerRequest.Header.Set("Authorization", "Bearer bearer_token_123")
	if token := ExtractRequestSessionToken(bearerRequest); token != "bearer_token_123" {
		t.Fatalf("expected bearer_token_123, got %s", token)
	}

	bearerLowerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerLowerRequest.Header.Set("Authorization", "bearer bearer_token_lower")
	if token := ExtractRequestSessionToken(bearerLowerRequest); token != "bearer_token_lower" {
		t.Fatalf("expected bearer_token_lower, got %s", token)
	}

	secureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	secureCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameSecure, Value: "secure_cookie_value"})
	if token := ExtractRequestSessionToken(secureCookieRequest); token != "secure_cookie_value" {
		t.Fatalf("expected secure_cookie_value, got %s", token)
	}

	plainCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	plainCookieRequest.AddCookie(&http.Cookie{Name: SessionCookieNameInsecure, Value: "plain_cookie_value"})
	if token := ExtractRequestSessionToken(plainCookieRequest); token != "plain_cookie_value" {
		t.Fatalf("expected plain_cookie_value, got %s", token)
	}

	emptyTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if token := ExtractRequestSessionToken(emptyTokenRequest); token != "" {
		t.Fatalf("expected empty token, got %s", token)
	}

	// 5. AuthContext WithAuthContext and GetAuthContext tests
	ctx := context.Background()
	emptyAuthContext := GetAuthContext(ctx)
	if emptyAuthContext.UserID != "" || emptyAuthContext.ServiceAccountID != "" || emptyAuthContext.IsAuthenticated() || emptyAuthContext.IsUser() || emptyAuthContext.IsServiceAccount() {
		t.Fatalf("expected empty unauthenticated state on empty context: %+v", emptyAuthContext)
	}
	var nilCtx context.Context //nolint:staticcheck
	if nilCtxAuthContext := GetAuthContext(nilCtx); nilCtxAuthContext.UserID != "" || nilCtxAuthContext.IsAuthenticated() {
		t.Fatal("expected empty UserID on nil context")
	}

	expectedAuthContext := AuthContext{
		JWT: JWTClaims{
			Subject:   "usr_abc",
			SessionID: "sess_xyz",
			Role:      "authenticated",
		},
		RefreshTokenHash: "hash123",
	}
	authCtx := WithAuthContext(ctx, expectedAuthContext)
	retrievedAuthContext := GetAuthContext(authCtx)
	if retrievedAuthContext.UserID != "usr_abc" || retrievedAuthContext.ServiceAccountID != "" || retrievedAuthContext.Role() != "authenticated" || retrievedAuthContext.JWT.Subject != "usr_abc" || retrievedAuthContext.JWT.SessionID != "sess_xyz" || retrievedAuthContext.RefreshTokenHash != "hash123" {
		t.Fatalf("unexpected retrieved auth context: %+v", retrievedAuthContext)
	}
	if !retrievedAuthContext.IsAuthenticated() || !retrievedAuthContext.IsUser() || retrievedAuthContext.IsServiceAccount() {
		t.Fatalf("expected user auth context flags: %+v", retrievedAuthContext)
	}

	// Direct user context without JWT
	directUserCtx := WithAuthContext(ctx, AuthContext{UserID: "usr_direct"})
	retrievedDirectUserAuthContext := GetAuthContext(directUserCtx)
	if retrievedDirectUserAuthContext.UserID != "usr_direct" || retrievedDirectUserAuthContext.ServiceAccountID != "" || !retrievedDirectUserAuthContext.IsAuthenticated() || !retrievedDirectUserAuthContext.IsUser() || retrievedDirectUserAuthContext.IsServiceAccount() || retrievedDirectUserAuthContext.Role() != "authenticated" {
		t.Fatalf("unexpected direct user context: %+v", retrievedDirectUserAuthContext)
	}

	// Service account isolation test: service account must NEVER have UserID populated!
	serviceAccountAuthContext := AuthContext{
		ServiceAccountID: "sa_xyz",
		JWT: JWTClaims{
			Subject:  "sa_xyz",
			Role:     "service_role",
			Audience: "app:service_account",
			Scope:    "data:read auth:write",
		},
	}
	serviceAccountCtx := WithAuthContext(ctx, serviceAccountAuthContext)
	retrievedServiceAccountAuthContext := GetAuthContext(serviceAccountCtx)
	if retrievedServiceAccountAuthContext.UserID != "" {
		t.Fatalf("service account must never have UserID populated: got %q", retrievedServiceAccountAuthContext.UserID)
	}
	if retrievedServiceAccountAuthContext.ServiceAccountID != "sa_xyz" || retrievedServiceAccountAuthContext.Role() != "service_role" {
		t.Fatalf("unexpected service account context: %+v", retrievedServiceAccountAuthContext)
	}
	if !retrievedServiceAccountAuthContext.HasScope("data:read") || retrievedServiceAccountAuthContext.HasScope("data:delete") {
		t.Fatalf("unexpected scope evaluation on service account auth context: %+v", retrievedServiceAccountAuthContext)
	}
	if !retrievedServiceAccountAuthContext.IsAuthenticated() || retrievedServiceAccountAuthContext.IsUser() || !retrievedServiceAccountAuthContext.IsServiceAccount() {
		t.Fatalf("expected service account auth flags: %+v", retrievedServiceAccountAuthContext)
	}

	// Service account by role with empty ServiceAccountID and accidental UserID should clear UserID and populate ServiceAccountID
	serviceAccountByRoleAuthContext := AuthContext{
		UserID: "accidental_user_id",
		JWT: JWTClaims{
			Subject: "sa_from_subject",
			Role:    "service_role",
		},
	}
	serviceAccountRoleCtx := WithAuthContext(ctx, serviceAccountByRoleAuthContext)
	retrievedServiceAccountRoleAuthContext := GetAuthContext(serviceAccountRoleCtx)
	if retrievedServiceAccountRoleAuthContext.UserID != "" {
		t.Fatalf("service account must have UserID cleared: got %q", retrievedServiceAccountRoleAuthContext.UserID)
	}
	if retrievedServiceAccountRoleAuthContext.ServiceAccountID != "sa_from_subject" {
		t.Fatalf("expected ServiceAccountID sa_from_subject, got %q", retrievedServiceAccountRoleAuthContext.ServiceAccountID)
	}

	// Service account with empty JWT.Role defaults to "service_role"
	serviceAccountEmptyRoleCtx := WithAuthContext(ctx, AuthContext{
		ServiceAccountID: "sa_empty_role",
	})
	retrievedServiceAccountEmptyRoleAuthContext := GetAuthContext(serviceAccountEmptyRoleCtx)
	if retrievedServiceAccountEmptyRoleAuthContext.Role() != "service_role" || retrievedServiceAccountEmptyRoleAuthContext.JWT.Role != "service_role" {
		t.Fatalf("expected role service_role, got: %+v", retrievedServiceAccountEmptyRoleAuthContext)
	}

	// User with empty JWT.Role defaults to "authenticated"
	userEmptyRoleCtx := WithAuthContext(ctx, AuthContext{
		JWT: JWTClaims{
			Subject: "usr_empty_role",
		},
	})
	retrievedUserEmptyRoleAuthContext := GetAuthContext(userEmptyRoleCtx)
	if retrievedUserEmptyRoleAuthContext.Role() != "authenticated" || retrievedUserEmptyRoleAuthContext.JWT.Role != "authenticated" {
		t.Fatalf("expected role authenticated, got: %+v", retrievedUserEmptyRoleAuthContext)
	}
}
