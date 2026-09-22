package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"uuid"
)

func TestCoreEventHookValidateDriverUnit(t *testing.T) {
	if err := validateHookDriver("sql"); err != nil {
		t.Fatalf("expected sql driver to be valid, got: %v", err)
	}
	if err := validateHookDriver("http"); err != nil {
		t.Fatalf("expected http driver to be valid, got: %v", err)
	}
	if err := validateHookDriver("grpc"); err == nil {
		t.Fatal("expected error for invalid driver grpc")
	}
	if err := validateHookDriver(""); err == nil {
		t.Fatal("expected error for empty driver")
	}
}

func TestCoreEventHookValidateSQLFunctionNameUnit(t *testing.T) {
	testCases := []struct {
		name         string
		functionName string
		shouldError  bool
	}{
		{"valid unqualified", "my_function", false},
		{"valid schema qualified", "public.my_function", false},
		{"valid custom schema", "custom_schema.sync_data_123", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"sql injection attempt semicolon", "public.fn; DROP TABLE core.events;", true},
		{"sql injection attempt quotes", "public.\"fn\"()", true},
		{"invalid character dash", "public.my-func", true},
		{"too many dots", "a.b.c", true},
		{"starts with number", "123func", true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateSQLFunctionName(testCase.functionName)
			if testCase.shouldError && err == nil {
				t.Fatalf("expected error for functionName %q, got nil", testCase.functionName)
			}
			if !testCase.shouldError && err != nil {
				t.Fatalf("expected valid functionName %q, got error: %v", testCase.functionName, err)
			}
		})
	}
}

func TestCoreEventHookValidateTargetURLUnit(t *testing.T) {
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
			err := validateHookTargetURL(testCase.targetURL)
			if testCase.shouldError && err == nil {
				t.Fatalf("expected error for URL %q, got nil", testCase.targetURL)
			}
			if !testCase.shouldError && err != nil {
				t.Fatalf("expected valid URL %q, got error: %v", testCase.targetURL, err)
			}
		})
	}
}

func TestCoreEventHookComputeSignatureUnit(t *testing.T) {
	secret := "hook_secret_key_123"
	payload := []byte(`{"event":"test"}`)
	unixTimestamp := time.Now().Unix()
	timestamp := fmt.Sprintf("%d", unixTimestamp)

	signature := ComputeHookSignature(secret, timestamp, payload)
	if len(signature) == 0 {
		t.Fatal("expected non-empty HMAC signature")
	}

	// Deterministic
	signatureSecond := ComputeHookSignature(secret, timestamp, payload)
	if signature != signatureSecond {
		t.Fatal("expected deterministic HMAC signature computation")
	}

	// Different timestamp -> different signature
	differentTimestamp := fmt.Sprintf("%d", unixTimestamp+10)
	signatureDifferentTime := ComputeHookSignature(secret, differentTimestamp, payload)
	if signature == signatureDifferentTime {
		t.Fatal("expected different signature for different timestamp")
	}
}

func TestCoreEventHookTransientErrorUnit(t *testing.T) {
	if isTransientDatabaseError(nil) {
		t.Fatal("expected nil error to not be transient")
	}

	transientErrors := []string{
		"connection refused",
		"statement timeout: 57014",
		"deadlock detected: 40p01",
		"could not serialize access: 40001",
		"write: broken pipe",
		"connection reset by peer",
		"server closed the connection",
	}

	for _, errPattern := range transientErrors {
		if !isTransientDatabaseError(errors.New(errPattern)) {
			t.Fatalf("expected error %q to be classified as transient", errPattern)
		}
	}

	permanentErrors := []string{
		"syntax error at or near \"SELECT\"",
		"function public.non_existent() does not exist",
		"relation \"missing_table\" does not exist",
	}

	for _, errPattern := range permanentErrors {
		if isTransientDatabaseError(errors.New(errPattern)) {
			t.Fatalf("expected error %q to be classified as permanent", errPattern)
		}
	}
}

func TestCoreEventHookSleepWithContextUnit(t *testing.T) {
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

func TestCoreEventHookManagerValidationUnit(t *testing.T) {
	eventHookManager := NewEventHookManager(nil, nil)
	ctx := context.Background()

	// 1. Create validations
	// Empty name
	_, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:   "",
		Driver: EventHookDriverHTTP,
	})
	if err == nil {
		t.Fatal("expected error on empty Name")
	}

	// Invalid driver
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:   "Hook",
		Driver: "invalid-driver",
	})
	if err == nil {
		t.Fatal("expected error on invalid driver")
	}

	// SQL driver missing sql_function_name
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:   "SQL Hook",
		Driver: EventHookDriverSQL,
	})
	if err == nil {
		t.Fatal("expected error on missing sql_function_name")
	}

	// SQL driver invalid sql_function_name
	invalidFn := "invalid fn name;"
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &invalidFn,
	})
	if err == nil {
		t.Fatal("expected error on invalid sql_function_name")
	}

	// SQL driver with http_target_url
	validFn := "public.test_fn"
	targetURL := "https://example.com"
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &validFn,
		HTTPTargetURL:   &targetURL,
	})
	if err == nil {
		t.Fatal("expected error when providing http_target_url for sql driver")
	}

	// SQL driver with signing_secret
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &validFn,
		SigningSecret:   "secret",
	})
	if err == nil {
		t.Fatal("expected error when providing signing_secret for sql driver")
	}

	// HTTP driver missing http_target_url
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:   "HTTP Hook",
		Driver: EventHookDriverHTTP,
	})
	if err == nil {
		t.Fatal("expected error on missing http_target_url")
	}

	// HTTP driver invalid target url
	badURL := "ftp://example.com"
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "HTTP Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &badURL,
	})
	if err == nil {
		t.Fatal("expected error on invalid http_target_url")
	}

	// HTTP driver with sql_function_name
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "HTTP Hook",
		Driver:          EventHookDriverHTTP,
		HTTPTargetURL:   &targetURL,
		SQLFunctionName: &validFn,
	})
	if err == nil {
		t.Fatal("expected error when providing sql_function_name for http driver")
	}

	// HTTP driver with signing_secret and failing cryptoKeyManager
	failingCryptoKeyManager, cryptoKeyErr := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if cryptoKeyErr != nil {
		t.Fatalf("unexpected cryptoKeyManager init error: %v", cryptoKeyErr)
	}
	failingCryptoKeyManager.randomReader = &simulatedFailingReader{}
	failingCryptoEventHookManager := NewEventHookManager(nil, failingCryptoKeyManager)
	_, err = failingCryptoEventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "HTTP Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &targetURL,
		SigningSecret: "secret",
	})
	if err == nil {
		t.Fatal("expected error when EncryptField fails in Create")
	}

	// Empty event types
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "HTTP Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &targetURL,
		EventTypes:    []string{},
	})
	if err == nil {
		t.Fatal("expected error on empty event_types")
	}

	// Pattern with empty string
	_, err = eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "HTTP Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &targetURL,
		EventTypes:    []string{"  "},
	})
	if err == nil {
		t.Fatal("expected error on empty pattern in event_types")
	}

	// executeSQLAttempt error branches
	if _, _, err := eventHookManager.executeSQLAttempt(ctx, EventHook{Driver: EventHookDriverSQL}, []byte(`{}`), 10); err == nil {
		t.Fatal("expected error on nil SQLFunctionName")
	}

	// executeHTTPAttempt error branches
	fakeID := uuid.NewV7()
	if _, _, _, err := eventHookManager.executeHTTPAttempt(ctx, EventHook{Driver: EventHookDriverHTTP}, Event{}, []byte(`{}`), fakeID, 10); err == nil {
		t.Fatal("expected error on nil HTTPTargetURL")
	}
	invalidRequestURL := "http://invalid\x7furl"
	if _, _, _, err := eventHookManager.executeHTTPAttempt(ctx, EventHook{Driver: EventHookDriverHTTP, HTTPTargetURL: &invalidRequestURL}, Event{}, []byte(`{}`), fakeID, 10); err == nil {
		t.Fatal("expected error on invalid target URL generating request error")
	}
}
