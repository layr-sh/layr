package threat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThreatCaptchaFactoryUnit(t *testing.T) {
	client := &mockHTTPClient{}

	// Missing secret key
	if _, err := NewCaptchaVerifier("turnstile", "", client); !errors.Is(err, ErrMissingCaptchaSecretKey) {
		t.Fatalf("expected ErrMissingCaptchaSecretKey, got %v", err)
	}

	// Supported providers
	providers := []struct {
		name         string
		expectedType string
	}{
		{"", "*threat.TurnstileVerifier"},
		{"turnstile", "*threat.TurnstileVerifier"},
		{"cloudflare", "*threat.TurnstileVerifier"},
		{"recaptcha", "*threat.RecaptchaVerifier"},
		{"google", "*threat.RecaptchaVerifier"},
		{"hcaptcha", "*threat.HCaptchaVerifier"},
	}

	for _, providerTestCase := range providers {
		captchaVerifier, err := NewCaptchaVerifier(providerTestCase.name, "secret-key", client)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", providerTestCase.name, err)
		}
		if captchaVerifier == nil {
			t.Fatalf("expected non-nil verifier for %s", providerTestCase.name)
		}
	}

	// Unsupported provider
	if _, err := NewCaptchaVerifier("unsupported", "secret-key", client); !errors.Is(err, ErrUnsupportedCaptchaProvider) {
		t.Fatalf("expected ErrUnsupportedCaptchaProvider, got %v", err)
	}
}

func TestThreatCaptchaEmptyTokenUnit(t *testing.T) {
	ctx := context.Background()
	client := &mockHTTPClient{}

	turnstileCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", client)
	recaptchaCaptchaVerifier, _ := NewCaptchaVerifier("recaptcha", "secret", client)
	hCaptchaCaptchaVerifier, _ := NewCaptchaVerifier("hcaptcha", "secret", client)

	for _, captchaVerifier := range []CaptchaVerifier{turnstileCaptchaVerifier, recaptchaCaptchaVerifier, hCaptchaCaptchaVerifier} {
		valid, err := captchaVerifier.Verify(ctx, "   ", "127.0.0.1")
		if err != nil || valid {
			t.Fatalf("expected (false, nil) for empty token, got (%v, %v)", valid, err)
		}
	}
}

func TestThreatCaptchaVerifySuccessAndFailureUnit(t *testing.T) {
	ctx := context.Background()

	// Success response
	successClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"success": true}`)),
			}, nil
		},
	}

	turnstileCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", successClient)
	recaptchaCaptchaVerifier, _ := NewCaptchaVerifier("recaptcha", "secret", successClient)
	hCaptchaCaptchaVerifier, _ := NewCaptchaVerifier("hcaptcha", "secret", successClient)

	for _, captchaVerifier := range []CaptchaVerifier{turnstileCaptchaVerifier, recaptchaCaptchaVerifier, hCaptchaCaptchaVerifier} {
		valid, err := captchaVerifier.Verify(ctx, "valid-token", "1.2.3.4")
		if err != nil || !valid {
			t.Fatalf("expected valid token, got (%v, %v)", valid, err)
		}
	}

	// Without remote IP
	validNoIP, noIPErr := turnstileCaptchaVerifier.Verify(ctx, "valid-token", "")
	if noIPErr != nil || !validNoIP {
		t.Fatalf("expected valid token without IP, got (%v, %v)", validNoIP, noIPErr)
	}

	// Failure response
	failureClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"success": false, "error-codes": ["invalid-input-response"]}`)),
			}, nil
		},
	}
	turnstileFailCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", failureClient)
	validFail, failErr := turnstileFailCaptchaVerifier.Verify(ctx, "invalid-token", "1.2.3.4")
	if failErr != nil || validFail {
		t.Fatalf("expected invalid token, got (%v, %v)", validFail, failErr)
	}
}

func TestThreatCaptchaErrorsUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Request creation failure (nil context)
	var nilCtx context.Context
	turnstileCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", &mockHTTPClient{})
	if _, err := turnstileCaptchaVerifier.Verify(nilCtx, "token", "1.2.3.4"); err == nil {
		t.Fatal("expected error with nil context")
	}

	// 2. HTTP client error
	failingClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}
	failingCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", failingClient)
	if _, err := failingCaptchaVerifier.Verify(ctx, "token", "1.2.3.4"); err == nil {
		t.Fatal("expected error when HTTP client fails")
	}

	// 3. Non-200 status code
	status500Client := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
			}, nil
		},
	}
	status500CaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", status500Client)
	if _, err := status500CaptchaVerifier.Verify(ctx, "token", "1.2.3.4"); err == nil {
		t.Fatal("expected error on non-200 status code")
	}

	// 4. Invalid JSON
	invalidJSONClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not json")),
			}, nil
		},
	}
	invalidJSONCaptchaVerifier, _ := NewCaptchaVerifier("turnstile", "secret", invalidJSONClient)
	if _, err := invalidJSONCaptchaVerifier.Verify(ctx, "token", "1.2.3.4"); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestThreatCaptchaDefaultClientUnit(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"success": true}`))
	}))
	defer testServer.Close()

	originalURL := turnstileVerifyURL
	turnstileVerifyURL = testServer.URL
	defer func() {
		turnstileVerifyURL = originalURL
	}()

	defaultCaptchaVerifier, err := NewCaptchaVerifier("turnstile", "secret", nil)
	if err != nil {
		t.Fatalf("unexpected error creating verifier: %v", err)
	}

	valid, verifyErr := defaultCaptchaVerifier.Verify(context.Background(), "token", "127.0.0.1")
	if verifyErr != nil || !valid {
		t.Fatalf("expected valid with default client, got (%v, %v)", valid, verifyErr)
	}
}
