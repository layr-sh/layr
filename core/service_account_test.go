package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreServiceAccountContextHelpersUnit(t *testing.T) {
	ctx := context.Background()

	// Initial context has no service account
	if serviceAccount := GetServiceAccount(ctx); serviceAccount != nil {
		t.Fatalf("expected nil service account from background context, got %+v", serviceAccount)
	}

	testServiceAccount := &ServiceAccount{
		ID:        "sa-123",
		Name:      "test-service-account",
		KeyPrefix: "a1b2c3d4",
		Scopes:    []string{"data:*", "auth:user.read"},
		IsEnabled: true,
	}

	withServiceAccountCtx := WithServiceAccount(ctx, testServiceAccount)
	extractedServiceAccount := GetServiceAccount(withServiceAccountCtx)
	if extractedServiceAccount == nil || extractedServiceAccount.ID != "sa-123" {
		t.Fatalf("expected extracted service account ID sa-123, got %+v", extractedServiceAccount)
	}
}

func TestCoreServiceAccountIsIPAllowedUnit(t *testing.T) {
	// Invalid client IP
	if isIPAllowed("invalid-ip", []string{"192.168.1.1"}) {
		t.Fatal("expected invalid client IP to be disallowed")
	}

	// Empty patterns returns false
	if isIPAllowed("192.168.1.1", nil) {
		t.Fatal("expected nil allowed patterns to return false")
	}
	if isIPAllowed("192.168.1.1", []string{}) {
		t.Fatal("expected empty allowed patterns to return false")
	}

	// Exact IP match
	if !isIPAllowed("10.0.0.1", []string{"10.0.0.1", "10.0.0.2"}) {
		t.Fatal("expected exact IP match to be allowed")
	}
	if isIPAllowed("10.0.0.3", []string{"10.0.0.1", "10.0.0.2"}) {
		t.Fatal("expected non-matching IP to be disallowed")
	}

	// CIDR match
	if !isIPAllowed("192.168.1.50", []string{"192.168.1.0/24"}) {
		t.Fatal("expected IP in CIDR block to be allowed")
	}
	if isIPAllowed("192.168.2.50", []string{"192.168.1.0/24"}) {
		t.Fatal("expected IP outside CIDR block to be disallowed")
	}

	// Malformed CIDR and invalid string patterns
	if isIPAllowed("192.168.1.1", []string{"invalid/cidr/string"}) {
		t.Fatal("expected invalid CIDR pattern to be ignored")
	}
	if isIPAllowed("192.168.1.1", []string{"not-an-ip"}) {
		t.Fatal("expected invalid IP string pattern to be ignored")
	}
}

func TestCoreServiceAccountGenerateSecretKeyUnit(t *testing.T) {
	prefix, secretKey, keyHash := GenerateServiceAccountSecretKey()

	if len(secretKey) == 0 || len(prefix) == 0 || len(keyHash) == 0 {
		t.Fatal("expected non-empty key components")
	}
	if len(prefix) != 8 {
		t.Fatalf("expected 8-char prefix, got %d", len(prefix))
	}
}

func TestCoreServiceAccountExtractRequestKeyUnit(t *testing.T) {
	// ExtractRequestServiceAccountKey from X-Layr-Service-Account-Key header
	keyHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	keyHeaderRequest.Header.Set("X-Layr-Service-Account-Key", "a1b2c3d4e5f6789012345678abcdef01")
	if key := ExtractRequestServiceAccountKey(keyHeaderRequest); key != "a1b2c3d4e5f6789012345678abcdef01" {
		t.Fatalf("expected a1b2c3d4e5f6789012345678abcdef01, got %s", key)
	}

	// ExtractRequestServiceAccountKey from X-Service-Account-Key header
	shortKeyHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	shortKeyHeaderRequest.Header.Set("X-Service-Account-Key", "fedcba9876543210")
	if key := ExtractRequestServiceAccountKey(shortKeyHeaderRequest); key != "fedcba9876543210" {
		t.Fatalf("expected fedcba9876543210, got %s", key)
	}

	// ExtractRequestServiceAccountKey from Authorization: Bearer
	bearerHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerHeaderRequest.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	if key := ExtractRequestServiceAccountKey(bearerHeaderRequest); key != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("expected 0123456789abcdef0123456789abcdef, got %s", key)
	}

	// ExtractRequestServiceAccountKey with no headers
	noHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if key := ExtractRequestServiceAccountKey(noHeaderRequest); key != "" {
		t.Fatalf("expected empty key, got %s", key)
	}
}
