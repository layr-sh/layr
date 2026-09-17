package common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
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

func TestCommonClaimsValidationAndMappingUnit(t *testing.T) {
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

	// 2. BuildClaimsMap with all standard and custom claims (including SessionID -> sid)
	fullJWTClaims := core.JWTClaims{
		Subject:     "usr_12345",
		SessionID:   "session_uuid_123",
		Role:        "editor",
		Issuer:      "layr-app",
		Audience:    "layr-app:user",
		ExpiresAt:   1750000000,
		NotBefore:   1700000000,
		IssuedAt:    1700000001,
		JWTID:       "token_uuid_001",
		Email:       "alice@example.com",
		Phone:       "+1234567890",
		IsAnonymous: true,
		Scope:       "read write admin",
		Claims: map[string]any{
			"org":       "org_acme",
			"tenant-id": "tenant_001",
			"bad!claim": "should_be_ignored",
		},
	}

	claimsMap := BuildClaimsMap(fullJWTClaims)
	if claimsMap["sub"] != "usr_12345" {
		t.Fatalf("expected sub usr_12345, got %v", claimsMap["sub"])
	}
	if claimsMap["sid"] != "session_uuid_123" {
		t.Fatalf("expected sid session_uuid_123, got %v", claimsMap["sid"])
	}
	if claimsMap["role"] != "editor" {
		t.Fatalf("expected role editor, got %v", claimsMap["role"])
	}
	if claimsMap["iss"] != "layr-app" {
		t.Fatalf("expected iss layr-app, got %v", claimsMap["iss"])
	}
	if claimsMap["aud"] != "layr-app:user" {
		t.Fatalf("expected aud layr-app:user, got %v", claimsMap["aud"])
	}
	if claimsMap["exp"] != int64(1750000000) {
		t.Fatalf("expected exp 1750000000, got %v", claimsMap["exp"])
	}
	if claimsMap["nbf"] != int64(1700000000) {
		t.Fatalf("expected nbf 1700000000, got %v", claimsMap["nbf"])
	}
	if claimsMap["iat"] != int64(1700000001) {
		t.Fatalf("expected iat 1700000001, got %v", claimsMap["iat"])
	}
	if claimsMap["jti"] != "token_uuid_001" {
		t.Fatalf("expected jti token_uuid_001, got %v", claimsMap["jti"])
	}
	if claimsMap["email"] != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %v", claimsMap["email"])
	}
	if claimsMap["phone"] != "+1234567890" {
		t.Fatalf("expected phone +1234567890, got %v", claimsMap["phone"])
	}
	if claimsMap["is_anonymous"] != true {
		t.Fatalf("expected is_anonymous true, got %v", claimsMap["is_anonymous"])
	}
	if claimsMap["scope"] != "read write admin" {
		t.Fatalf("expected scope 'read write admin', got %v", claimsMap["scope"])
	}
	if claimsMap["org"] != "org_acme" {
		t.Fatalf("expected org org_acme, got %v", claimsMap["org"])
	}
	if claimsMap["tenant-id"] != "tenant_001" {
		t.Fatalf("expected tenant-id tenant_001, got %v", claimsMap["tenant-id"])
	}
	if _, exists := claimsMap["bad!claim"]; exists {
		t.Fatal("expected unsafe claim bad!claim to be excluded from BuildClaimsMap")
	}

	// 3. ApplyRLS with stub transaction in isolation covering various claim types
	ctx := context.Background()
	mockStubTransaction := &stubTransaction{}

	populatedJWTClaims := core.JWTClaims{
		Subject:     "usr_999",
		SessionID:   "sess_999",
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
		Scope:       "read:data write:data",
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

	ApplyRLS(ctx, mockStubTransaction, populatedJWTClaims)

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
		"request.jwt.sid":          "sess_999",
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
		"request.jwt.scope":        "read:data write:data",
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

	// 4. Test unmarshalable claim fallback to fmt.Sprintf
	unmarshalableTransaction := &stubTransaction{}
	unmarshalableJWTClaims := core.JWTClaims{
		Claims: map[string]any{
			"chan_key": make(chan int),
		},
	}
	ApplyRLS(ctx, unmarshalableTransaction, unmarshalableJWTClaims)
	if len(unmarshalableTransaction.executions) == 0 {
		t.Fatal("expected execution for unmarshalable claim")
	}

	// 5. Empty claims invoke zero executions
	emptyStubTransaction := &stubTransaction{}
	ApplyRLS(ctx, emptyStubTransaction, core.JWTClaims{})
	if len(emptyStubTransaction.executions) != 0 {
		t.Fatalf("expected zero executions for empty claims, got %d", len(emptyStubTransaction.executions))
	}
}
