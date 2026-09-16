package common

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/auth/jwt"
)

type recordedExecution struct {
	query     string
	arguments []any
}

type stubTransaction struct {
	pgx.Tx
	executions []recordedExecution
}

func (s *stubTransaction) Exec(ctx context.Context, query string, arguments ...any) (pgconn.CommandTag, error) {
	s.executions = append(s.executions, recordedExecution{
		query:     query,
		arguments: arguments,
	})
	return pgconn.CommandTag{}, nil
}

func TestCommonClaimsExtractionAndValidationUnit(t *testing.T) {
	// 1. IsSafeClaimKey validation
	if IsSafeClaimKey("") {
		t.Fatal("expected empty claim key to be unsafe")
	}
	if IsSafeClaimKey(strings.Repeat("a", 64)) {
		t.Fatal("expected 64-char claim key to be unsafe")
	}
	if !IsSafeClaimKey(strings.Repeat("a", 63)) {
		t.Fatal("expected 63-char claim key to be safe")
	}

	validKeys := []string{
		"tenant_id",
		"tenant-id",
		"TenantID",
		"user123",
		"org_tier_2",
		"a",
		"Z",
		"0",
		"_",
		"-",
	}
	for _, key := range validKeys {
		if !IsSafeClaimKey(key) {
			t.Fatalf("expected claim key %q to be safe", key)
		}
	}

	invalidKeys := []string{
		"tenant.id",
		"tenant/id",
		"tenant!id",
		"tenant@id",
		"tenant:id",
		"tenant;id",
		"tenant id",
		"tenant'id",
		"tenant\"id",
		"tenant\x00id",
	}
	for _, key := range invalidKeys {
		if IsSafeClaimKey(key) {
			t.Fatalf("expected claim key %q to be unsafe", key)
		}
	}

	// 2. ExtractClaims with full X-JWT-* standard headers
	ctx := context.Background()
	fullRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/data/query", nil)
	fullRequest.Header.Set("X-JWT-Sub", "usr_12345")
	fullRequest.Header.Set("X-JWT-Role", "editor")
	fullRequest.Header.Set("X-JWT-Iss", "layr-app")
	fullRequest.Header.Set("X-JWT-Aud", "layr-app:user")
	fullRequest.Header.Set("X-JWT-Exp", "1750000000")
	fullRequest.Header.Set("X-JWT-Nbf", "1700000000")
	fullRequest.Header.Set("X-JWT-Iat", "1700000001")
	fullRequest.Header.Set("X-JWT-Jti", "token_uuid_001")
	fullRequest.Header.Set("X-JWT-Email", "alice@example.com")
	fullRequest.Header.Set("X-JWT-Phone", "+1234567890")
	fullRequest.Header.Set("X-JWT-Is-Anonymous", "true")
	fullRequest.Header.Set("X-JWT-Scopes", "read write admin")
	fullRequest.Header.Set("X-JWT-Claim-Org", "org_acme")
	fullRequest.Header.Set("X-JWT-Claim-Tenant-Id", "tenant_001")
	fullRequest.Header.Set("Authorization", "Bearer token")
	fullRequest.Header.Set("Content-Type", "application/json")

	fullAuthClaims := ExtractClaims(fullRequest)
	if fullAuthClaims.Subject != "usr_12345" {
		t.Fatalf("expected subject usr_12345, got %q", fullAuthClaims.Subject)
	}
	if fullAuthClaims.Role != "editor" {
		t.Fatalf("expected role editor, got %q", fullAuthClaims.Role)
	}
	if fullAuthClaims.Issuer != "layr-app" {
		t.Fatalf("expected issuer layr-app, got %q", fullAuthClaims.Issuer)
	}
	if fullAuthClaims.Audience != "layr-app:user" {
		t.Fatalf("expected audience layr-app:user, got %q", fullAuthClaims.Audience)
	}
	if fullAuthClaims.ExpiresAt != 1750000000 {
		t.Fatalf("expected exp 1750000000, got %d", fullAuthClaims.ExpiresAt)
	}
	if fullAuthClaims.NotBefore != 1700000000 {
		t.Fatalf("expected nbf 1700000000, got %d", fullAuthClaims.NotBefore)
	}
	if fullAuthClaims.IssuedAt != 1700000001 {
		t.Fatalf("expected iat 1700000001, got %d", fullAuthClaims.IssuedAt)
	}
	if fullAuthClaims.JWTID != "token_uuid_001" {
		t.Fatalf("expected jti token_uuid_001, got %q", fullAuthClaims.JWTID)
	}
	if fullAuthClaims.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %q", fullAuthClaims.Email)
	}
	if fullAuthClaims.Phone != "+1234567890" {
		t.Fatalf("expected phone +1234567890, got %q", fullAuthClaims.Phone)
	}
	if !fullAuthClaims.IsAnonymous {
		t.Fatal("expected is_anonymous to be true")
	}
	if len(fullAuthClaims.Scopes) != 3 || fullAuthClaims.Scopes[0] != "read" {
		t.Fatalf("expected 3 scopes, got %v", fullAuthClaims.Scopes)
	}
	if fullAuthClaims.Claims["org"] != "org_acme" {
		t.Fatalf("expected org claim org_acme, got %v", fullAuthClaims.Claims["org"])
	}
	if fullAuthClaims.Claims["tenant-id"] != "tenant_001" {
		t.Fatalf("expected tenant-id claim tenant_001, got %v", fullAuthClaims.Claims["tenant-id"])
	}

	// 3. ExtractClaims via X-JWT-Claim-* headers when standard headers are empty
	claimPrefixedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/data/query", nil)
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Sub", "usr_from_prefix")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Role", "viewer")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Iss", "prefix-issuer")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Aud", "prefix-audience")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Exp", "1800000000")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Nbf", "1700000002")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Iat", "1700000003")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Jti", "jti_from_prefix")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Email", "prefix@example.com")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Phone", "+987654321")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Is_Anonymous", "true")
	claimPrefixedRequest.Header.Set("X-JWT-Claim-Scopes", "scope1 scope2")

	prefixedAuthClaims := ExtractClaims(claimPrefixedRequest)
	if prefixedAuthClaims.Subject != "usr_from_prefix" {
		t.Fatalf("expected subject usr_from_prefix, got %q", prefixedAuthClaims.Subject)
	}
	if prefixedAuthClaims.Role != "viewer" {
		t.Fatalf("expected role viewer, got %q", prefixedAuthClaims.Role)
	}
	if prefixedAuthClaims.Issuer != "prefix-issuer" {
		t.Fatalf("expected issuer prefix-issuer, got %q", prefixedAuthClaims.Issuer)
	}
	if prefixedAuthClaims.Audience != "prefix-audience" {
		t.Fatalf("expected audience prefix-audience, got %q", prefixedAuthClaims.Audience)
	}
	if prefixedAuthClaims.ExpiresAt != 1800000000 {
		t.Fatalf("expected exp 1800000000, got %d", prefixedAuthClaims.ExpiresAt)
	}
	if prefixedAuthClaims.NotBefore != 1700000002 {
		t.Fatalf("expected nbf 1700000002, got %d", prefixedAuthClaims.NotBefore)
	}
	if prefixedAuthClaims.IssuedAt != 1700000003 {
		t.Fatalf("expected iat 1700000003, got %d", prefixedAuthClaims.IssuedAt)
	}
	if prefixedAuthClaims.JWTID != "jti_from_prefix" {
		t.Fatalf("expected jti jti_from_prefix, got %q", prefixedAuthClaims.JWTID)
	}
	if prefixedAuthClaims.Email != "prefix@example.com" {
		t.Fatalf("expected email prefix@example.com, got %q", prefixedAuthClaims.Email)
	}
	if prefixedAuthClaims.Phone != "+987654321" {
		t.Fatalf("expected phone +987654321, got %q", prefixedAuthClaims.Phone)
	}
	if !prefixedAuthClaims.IsAnonymous {
		t.Fatal("expected is_anonymous true from prefix")
	}
	if len(prefixedAuthClaims.Scopes) != 2 {
		t.Fatalf("expected 2 scopes from prefix, got %v", prefixedAuthClaims.Scopes)
	}

	// 4. ExtractClaims with empty and partial headers
	emptyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	emptyAuthClaims := ExtractClaims(emptyRequest)
	if emptyAuthClaims.Subject != "" || emptyAuthClaims.Role != "" || len(emptyAuthClaims.Claims) != 0 {
		t.Fatalf("expected empty claims, got %+v", emptyAuthClaims)
	}

	// Header with empty value list
	emptyValueRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	emptyValueRequest.Header["X-JWT-Claim-Empty"] = []string{}
	emptyValueAuthClaims := ExtractClaims(emptyValueRequest)
	if len(emptyValueAuthClaims.Claims) != 0 {
		t.Fatalf("expected no claims for empty value list, got %+v", emptyValueAuthClaims.Claims)
	}

	// 5. ApplyRLS with stub transaction in isolation covering various claim types
	mockStubTransaction := &stubTransaction{}

	populatedAuthClaims := jwt.Claims{
		Subject:     "usr_999",
		Role:        "viewer",
		Issuer:      "layr-corp",
		Audience:    "layr-corp:viewer",
		ExpiresAt:   1799999999,
		NotBefore:   1700000000,
		IssuedAt:    1700000005,
		JWTID:       "jwt_unique_id",
		Email:       "viewer@layr.sh",
		Phone:       "+1000000000",
		IsAnonymous: true,
		Scopes:      []string{"read:data", "write:data"},
		Claims: map[string]any{
			"team_id":      "team_alpha",
			"score_int":    42,
			"score_int64":  int64(9999999999),
			"score_float":  3.14159,
			"is_active":    true,
			"tags":         []string{"tag1", "tag2"},
			"metadata_obj": map[string]string{"env": "prod"},
			"bad!claim":    "ignored_value",
		},
	}

	ApplyRLS(ctx, mockStubTransaction, populatedAuthClaims)

	appliedSettings := make(map[string]string)
	foundFullJSON := false

	for _, execution := range mockStubTransaction.executions {
		if execution.query == "SELECT set_config($1, $2, true)" && len(execution.arguments) == 2 {
			settingName := fmt.Sprintf("%v", execution.arguments[0])
			settingValue := fmt.Sprintf("%v", execution.arguments[1])
			appliedSettings[settingName] = settingValue
		}
		if execution.query == "SELECT set_config('request.jwt', $1, true)" && len(execution.arguments) == 1 {
			jsonPayload := fmt.Sprintf("%v", execution.arguments[0])
			if strings.Contains(jsonPayload, "team_alpha") && strings.Contains(jsonPayload, "usr_999") {
				foundFullJSON = true
			}
		}
	}

	expectedSettings := map[string]string{
		"request.jwt.sub":          "usr_999",
		"request.jwt.role":         "viewer",
		"request.jwt.iss":          "layr-corp",
		"request.jwt.aud":          "layr-corp:viewer",
		"request.jwt.exp":          "1799999999",
		"request.jwt.nbf":          "1700000000",
		"request.jwt.iat":          "1700000005",
		"request.jwt.jti":          "jwt_unique_id",
		"request.jwt.email":        "viewer@layr.sh",
		"request.jwt.phone":        "+1000000000",
		"request.jwt.is_anonymous": "true",
		"request.jwt.scopes":       "read:data write:data",
		"request.jwt.team_id":      "team_alpha",
		"request.jwt.score_int":    "42",
		"request.jwt.score_int64":  "9999999999",
		"request.jwt.score_float":  "3.14159",
		"request.jwt.is_active":    "true",
		"request.jwt.tags":         "tag1 tag2",
	}

	for key, expectedSettingValue := range expectedSettings {
		if actualSettingValue, exists := appliedSettings[key]; !exists || actualSettingValue != expectedSettingValue {
			t.Fatalf("expected setting %s to be %q, got %q (exists: %v)", key, expectedSettingValue, actualSettingValue, exists)
		}
	}

	if _, exists := appliedSettings["request.jwt.bad!claim"]; exists {
		t.Fatal("expected unsafe claim bad!claim to be ignored")
	}

	if !foundFullJSON {
		t.Fatal("expected request.jwt full JSON setting to be recorded")
	}

	// 6. Test unmarshalable claim fallback to fmt.Sprintf
	unmarshalableTransaction := &stubTransaction{}
	unmarshalableClaims := jwt.Claims{
		Claims: map[string]any{
			"chan_key": make(chan int),
		},
	}
	ApplyRLS(ctx, unmarshalableTransaction, unmarshalableClaims)
	if len(unmarshalableTransaction.executions) == 0 {
		t.Fatal("expected execution for unmarshalable claim")
	}

	// 7. Empty claims invoke zero executions
	emptyStubTransaction := &stubTransaction{}
	ApplyRLS(ctx, emptyStubTransaction, jwt.Claims{})
	if len(emptyStubTransaction.executions) != 0 {
		t.Fatalf("expected zero executions for empty claims, got %d", len(emptyStubTransaction.executions))
	}
}
