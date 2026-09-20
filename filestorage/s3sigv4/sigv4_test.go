package s3sigv4

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestS3sigv4ErrorDefinitionsUnit(t *testing.T) {
	if ErrMissingAuthHeader == nil || ErrInvalidAlgorithm == nil || ErrInvalidAccessKeyID == nil || ErrSignatureDoesNotMatch == nil || ErrRequestExpired == nil {
		t.Fatal("expected non-nil error definitions")
	}
}

func TestS3sigv4ParseAuthorizationHeaderUnit(t *testing.T) {
	// 1. Invalid algorithm
	invalidAlgHeader := "BEARER mytoken"
	if _, err := parseAuthorizationHeader(invalidAlgHeader); err == nil || !strings.Contains(err.Error(), "AWS4-HMAC-SHA256") {
		t.Fatalf("expected ErrInvalidAlgorithm, got: %v", err)
	}

	// 2. Malformed Credential parameter
	malformedCredentialHeader := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260920/us-east-1/s3, SignedHeaders=host, Signature=abc"
	if _, err := parseAuthorizationHeader(malformedCredentialHeader); err == nil || !strings.Contains(err.Error(), "malformed Credential") {
		t.Fatalf("expected malformed Credential error, got: %v", err)
	}

	// 3. Incomplete parameters
	incompleteHeader := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260920/us-east-1/s3/aws4_request"
	if _, err := parseAuthorizationHeader(incompleteHeader); err == nil || !strings.Contains(err.Error(), "missing or invalid authorization header") {
		t.Fatalf("expected ErrMissingAuthHeader, got: %v", err)
	}

	// 4. Valid header with extra malformed part to test continue
	validHeader := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260920/us-east-1/s3/aws4_request, standalone-part, SignedHeaders=host;x-amz-date, Signature=fe5f80f77d5fa3becb037a24e6ecd0c7f07726590367ecd50b47cf6014c27138"
	authCredentials, err := parseAuthorizationHeader(validHeader)
	if err != nil {
		t.Fatalf("expected successful parsing, got: %v", err)
	}
	if authCredentials.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Fatalf("expected access key AKIAIOSFODNN7EXAMPLE, got: %s", authCredentials.AccessKeyID)
	}
	if authCredentials.Date != "20260920" || authCredentials.Region != "us-east-1" || authCredentials.Service != "s3" {
		t.Fatalf("unexpected credential fields: %+v", authCredentials)
	}
	if authCredentials.SignedHeaders != "host;x-amz-date" {
		t.Fatalf("unexpected signed headers: %s", authCredentials.SignedHeaders)
	}
}

func TestS3sigv4ParseQueryParamsUnit(t *testing.T) {
	// 1. Invalid algorithm
	invalidValues := url.Values{}
	invalidValues.Set("X-Amz-Algorithm", "HMAC-SHA1")
	if _, err := parseQueryParams(invalidValues); err == nil || !strings.Contains(err.Error(), "AWS4-HMAC-SHA256") {
		t.Fatalf("expected ErrInvalidAlgorithm, got: %v", err)
	}

	// 2. Malformed credential
	malformedValues := url.Values{}
	malformedValues.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	malformedValues.Set("X-Amz-Credential", "short/credential")
	if _, err := parseQueryParams(malformedValues); err == nil || !strings.Contains(err.Error(), "malformed X-Amz-Credential") {
		t.Fatalf("expected malformed credential error, got: %v", err)
	}

	// 3. Missing date/signature
	incompleteValues := url.Values{}
	incompleteValues.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	incompleteValues.Set("X-Amz-Credential", "AKIA/20260920/us-east-1/s3/aws4_request")
	if _, err := parseQueryParams(incompleteValues); err == nil || !strings.Contains(err.Error(), "missing or invalid authorization header") {
		t.Fatalf("expected ErrMissingAuthHeader, got: %v", err)
	}

	// 4. Valid query
	validValues := url.Values{}
	validValues.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	validValues.Set("X-Amz-Credential", "AKIA/20260920/us-east-1/s3/aws4_request")
	validValues.Set("X-Amz-Date", "20260920T120000Z")
	validValues.Set("X-Amz-SignedHeaders", "host")
	validValues.Set("X-Amz-Signature", "1234567890abcdef")

	authCredentials, err := parseQueryParams(validValues)
	if err != nil {
		t.Fatalf("expected successful query parsing, got: %v", err)
	}
	if authCredentials.AccessKeyID != "AKIA" || authCredentials.Signature != "1234567890abcdef" {
		t.Fatalf("unexpected query parsed result: %+v", authCredentials)
	}
}

func TestS3sigv4CanonicalHelpersUnit(t *testing.T) {
	// Query string sorting and escaping
	values := url.Values{}
	values.Set("prefix", "photos/")
	values.Set("delimiter", "/")
	values.Set("max-keys", "10")
	values.Set("X-Amz-Signature", "omit-me")

	canonicalQuery := buildCanonicalQueryString(values, true)
	if strings.Contains(canonicalQuery, "omit-me") {
		t.Fatal("expected X-Amz-Signature to be omitted in query auth canonical query")
	}
	expectedQuery := "delimiter=%2F&max-keys=10&prefix=photos%2F"
	if canonicalQuery != expectedQuery {
		t.Fatalf("expected %q, got %q", expectedQuery, canonicalQuery)
	}

	// Canonical headers building
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bucket/key", nil)
	request.Header.Set("X-Amz-Date", "20260920T120000Z")
	request.Host = "example.com"

	canonicalHeaders := buildCanonicalHeaders(request, "host;x-amz-date")
	expectedHeaders := "host:example.com\nx-amz-date:20260920T120000Z\n"
	if canonicalHeaders != expectedHeaders {
		t.Fatalf("expected %q, got %q", expectedHeaders, canonicalHeaders)
	}

	// Fallback to request.URL.Host when request.Host is empty
	fallbackRequest, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://fallback.example.com/bucket/key", nil)
	fallbackRequest.Host = ""
	fallbackHeaders := buildCanonicalHeaders(fallbackRequest, "host")
	if fallbackHeaders != "host:fallback.example.com\n" {
		t.Fatalf("expected fallback to request.URL.Host, got %q", fallbackHeaders)
	}
}

func TestS3sigv4NilDatabaseUnit(t *testing.T) {
	validator := NewValidator(nil, nil)
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bucket", nil)
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIA/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	request.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))

	_, err := validator.Validate(request)
	if err == nil {
		t.Fatal("expected error on validate with nil database pool")
	}
}

func TestS3sigv4ValidatePreDatabaseUnit(t *testing.T) {
	validator := NewValidator(nil, nil)
	ctx := context.Background()

	// 1. Missing auth headers and query params
	noAuthRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	if _, err := validator.Validate(noAuthRequest); !errors.Is(err, ErrMissingAuthHeader) {
		t.Fatalf("expected ErrMissingAuthHeader, got: %v", err)
	}

	// 2. Malformed Authorization header
	badAuthRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	badAuthRequest.Header.Set("Authorization", "INVALID algorithm")
	if _, err := validator.Validate(badAuthRequest); !errors.Is(err, ErrInvalidAlgorithm) {
		t.Fatalf("expected ErrInvalidAlgorithm, got: %v", err)
	}

	// 3. Query authentication with invalid algorithm
	badQueryRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket?X-Amz-Algorithm=INVALID", nil)
	if _, err := validator.Validate(badQueryRequest); !errors.Is(err, ErrInvalidAlgorithm) {
		t.Fatalf("expected ErrInvalidAlgorithm, got: %v", err)
	}

	// 4. Invalid timestamp format
	badTimestampRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	badTimestampRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIA/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	badTimestampRequest.Header.Set("X-Amz-Date", "invalid-timestamp")
	if _, err := validator.Validate(badTimestampRequest); !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp, got: %v", err)
	}

	// 5. Expired timestamp in the past (>15m)
	expiredPastRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	expiredPastRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIA/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	expiredPastRequest.Header.Set("X-Amz-Date", time.Now().Add(-20*time.Minute).UTC().Format("20060102T150405Z"))
	if _, err := validator.Validate(expiredPastRequest); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("expected ErrRequestExpired for past timestamp, got: %v", err)
	}

	// 6. Expired timestamp in the future (>15m)
	expiredFutureRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	expiredFutureRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIA/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	expiredFutureRequest.Header.Set("X-Amz-Date", time.Now().Add(20*time.Minute).UTC().Format("20060102T150405Z"))
	if _, err := validator.Validate(expiredFutureRequest); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("expected ErrRequestExpired for future timestamp, got: %v", err)
	}

	// 7. Valid RFC1123 Date header fallback
	rfc1123DateRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/bucket", nil)
	rfc1123DateRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIA/20260920/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	rfc1123DateRequest.Header.Set("Date", time.Now().UTC().Format(time.RFC1123))
	if _, err := validator.Validate(rfc1123DateRequest); err == nil {
		t.Fatal("expected error on validate with RFC1123 date and nil database pool")
	}

	// 8. Query auth with valid replay window
	validQueryRequest, _ := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://example.com/bucket?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIA/20260920/us-east-1/s3/aws4_request&X-Amz-Date="+time.Now().UTC().Format("20060102T150405Z")+"&X-Amz-SignedHeaders=host&X-Amz-Signature=abc",
		nil,
	)
	if _, err := validator.Validate(validQueryRequest); err == nil {
		t.Fatal("expected error on validate with query auth and nil database pool")
	}
}

func TestS3sigv4IPAllowedUnit(t *testing.T) {
	if !isIPAllowed("192.168.1.50", []string{}) {
		t.Fatal("expected empty allowed IPs to allow all")
	}
	if !isIPAllowed("192.168.1.50", []string{"192.168.1.50"}) {
		t.Fatal("expected exact IP match to be allowed")
	}
	if !isIPAllowed("10.0.0.5", []string{"10.0.0.0/24"}) {
		t.Fatal("expected CIDR match to be allowed")
	}
	if isIPAllowed("10.0.1.5", []string{"10.0.0.0/24"}) {
		t.Fatal("expected IP outside CIDR to be blocked")
	}
	if isIPAllowed("invalid-ip", []string{"10.0.0.0/24"}) {
		t.Fatal("expected invalid client IP to be blocked")
	}
}
