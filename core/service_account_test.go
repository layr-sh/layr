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

	ctxWithServiceAccount := WithServiceAccount(ctx, testServiceAccount)
	extractedServiceAccount := GetServiceAccount(ctxWithServiceAccount)
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
	keyHeaderRequest.Header.Set("X-Layr-Service-Account-Key", "sec_key_12345")
	if key := ExtractRequestServiceAccountKey(keyHeaderRequest); key != "sec_key_12345" {
		t.Fatalf("expected sec_key_12345, got %s", key)
	}

	// ExtractRequestServiceAccountKey from Authorization: Bearer
	bearerHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerHeaderRequest.Header.Set("Authorization", "Bearer sec_key_67890")
	if key := ExtractRequestServiceAccountKey(bearerHeaderRequest); key != "sec_key_67890" {
		t.Fatalf("expected sec_key_67890, got %s", key)
	}

	// ExtractRequestServiceAccountKey with no headers
	noHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if key := ExtractRequestServiceAccountKey(noHeaderRequest); key != "" {
		t.Fatalf("expected empty key, got %s", key)
	}
}
