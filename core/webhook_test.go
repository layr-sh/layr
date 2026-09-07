package core

import (
	"fmt"
	"testing"
	"time"
)

func TestCoreWebhookMatchesEventUnit(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		event         string
		expectedMatch bool
	}{
		{"wildcard all", "*", "auth.user.created", true},
		{"prefix wildcard", "auth.*", "auth.user.created", true},
		{"exact match", "auth.user.created", "auth.user.created", true},
		{"mismatch prefix", "data.*", "auth.user.created", false},
		{"suffix mismatch", "auth.user.deleted", "auth.user.created", false},
		{"exact prefix match without dot", "auth.*", "auth", true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if match := matchWebhookEventPattern(testCase.pattern, testCase.event); match != testCase.expectedMatch {
				t.Fatalf("expected matchWebhookEventPattern(%q, %q) = %v, got %v", testCase.pattern, testCase.event, testCase.expectedMatch, match)
			}
		})
	}
}

func TestCoreWebhookComputeSignatureUnit(t *testing.T) {
	secret := "whsec_test_secret_key_123"
	payload := []byte(`{"event":"test"}`)
	unixTimestamp := time.Now().Unix()
	timestamp := fmt.Sprintf("%d", unixTimestamp)

	signature := ComputeWebhookSignature(secret, timestamp, payload)
	if len(signature) == 0 {
		t.Fatal("expected non-empty HMAC signature")
	}

	// Deterministic
	signatureSecond := ComputeWebhookSignature(secret, timestamp, payload)
	if signature != signatureSecond {
		t.Fatal("expected deterministic HMAC signature computation")
	}

	// Different timestamp -> different signature
	differentTimestamp := fmt.Sprintf("%d", unixTimestamp+10)
	signatureDifferentTime := ComputeWebhookSignature(secret, differentTimestamp, payload)
	if signature == signatureDifferentTime {
		t.Fatal("expected different signature for different timestamp")
	}
}

func TestCoreWebhookCalculateBackoffUnit(t *testing.T) {
	backoffAttemptFirst := CalculateWebhookBackoff(1)
	backoffAttemptSecond := CalculateWebhookBackoff(2)
	backoffAttemptThird := CalculateWebhookBackoff(3)

	if backoffAttemptFirst <= 0 || backoffAttemptSecond <= backoffAttemptFirst || backoffAttemptThird <= backoffAttemptSecond {
		t.Fatalf("expected exponential increase in backoff: first=%v, second=%v, third=%v",
			backoffAttemptFirst, backoffAttemptSecond, backoffAttemptThird)
	}
}
