package core

import (
	"context"
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

func TestCoreWebhookValidateTargetURLUnit(t *testing.T) {
	testCases := []struct {
		name        string
		targetURL   string
		shouldError bool
	}{
		{"valid http", "http://example.com/webhook", false},
		{"valid https", "https://api.example.com/v1/hook", false},
		{"valid loopback ip", "http://127.0.0.1:8080/events", false},
		{"invalid url syntax", "://invalid-url", true},
		{"invalid scheme ftp", "ftp://example.com/hook", true},
		{"missing hostname", "http:///path-only", true},
		{"google metadata internal", "http://metadata.google.internal/computeMetadata/v1", true},
		{"metadata short name", "http://metadata/latest", true},
		{"link local unicast ipv4", "http://169.254.169.254/latest/meta-data", true},
		{"link local unicast ipv6", "http://[fe80::1]/test", true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateWebhookTargetURL(testCase.targetURL)
			if testCase.shouldError && err == nil {
				t.Fatalf("expected error for URL %q, got nil", testCase.targetURL)
			}
			if !testCase.shouldError && err != nil {
				t.Fatalf("expected valid URL %q, got error: %v", testCase.targetURL, err)
			}
		})
	}
}

func TestCoreWebhookSleepWithContextUnit(t *testing.T) {
	// 1. Cancelled context -> returns false
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepWithContext(canceledCtx, 10*time.Second) {
		t.Fatal("expected sleepWithContext to return false on canceled context")
	}

	// 2. Active context -> returns true after duration
	activeCtx := context.Background()
	if !sleepWithContext(activeCtx, 1*time.Millisecond) {
		t.Fatal("expected sleepWithContext to return true on active context")
	}
}
