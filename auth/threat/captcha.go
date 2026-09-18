package threat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CaptchaVerifier defines the interface for third-party CAPTCHA verification services.
type CaptchaVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) (bool, error)
}

var (
	turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	recaptchaVerifyURL = "https://www.google.com/recaptcha/api/siteverify"
	hCaptchaVerifyURL  = "https://hcaptcha.com/siteverify"

	defaultCaptchaTimeout = 5 * time.Second
)

var (
	// ErrUnsupportedCaptchaProvider indicates an unrecognized CAPTCHA provider was specified.
	ErrUnsupportedCaptchaProvider = errors.New("unsupported captcha provider")
	// ErrMissingCaptchaSecretKey indicates that the secret key is required for verification.
	ErrMissingCaptchaSecretKey = errors.New("captcha secret key is required")
)

// NewCaptchaVerifier creates a CaptchaVerifier instance for the given provider.
func NewCaptchaVerifier(provider string, secretKey string, httpClient HTTPClient) (CaptchaVerifier, error) {
	if secretKey == "" {
		return nil, ErrMissingCaptchaSecretKey
	}

	trimmedProvider := strings.ToLower(strings.TrimSpace(provider))
	switch trimmedProvider {
	case "", "turnstile", "cloudflare":
		return &TurnstileVerifier{secretKey: secretKey, httpClient: httpClient}, nil
	case "recaptcha", "google":
		return &RecaptchaVerifier{secretKey: secretKey, httpClient: httpClient}, nil
	case "hcaptcha":
		return &HCaptchaVerifier{secretKey: secretKey, httpClient: httpClient}, nil
	default:
		return nil, fmt.Errorf("%w: '%s'", ErrUnsupportedCaptchaProvider, provider)
	}
}

// TurnstileVerifier verifies tokens against Cloudflare Turnstile.
type TurnstileVerifier struct {
	secretKey  string
	httpClient HTTPClient
}

// Verify sends the token and IP to Cloudflare Turnstile for verification.
func (turnstileVerifier *TurnstileVerifier) Verify(ctx context.Context, token string, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	return verifyFormPost(ctx, turnstileVerifier.httpClient, turnstileVerifyURL, turnstileVerifier.secretKey, token, remoteIP)
}

// RecaptchaVerifier verifies tokens against Google reCAPTCHA.
type RecaptchaVerifier struct {
	secretKey  string
	httpClient HTTPClient
}

// Verify sends the token and IP to Google reCAPTCHA for verification.
func (recaptchaVerifier *RecaptchaVerifier) Verify(ctx context.Context, token string, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	return verifyFormPost(ctx, recaptchaVerifier.httpClient, recaptchaVerifyURL, recaptchaVerifier.secretKey, token, remoteIP)
}

// HCaptchaVerifier verifies tokens against hCaptcha.
type HCaptchaVerifier struct {
	secretKey  string
	httpClient HTTPClient
}

// Verify sends the token and IP to hCaptcha for verification.
func (hCaptchaVerifier *HCaptchaVerifier) Verify(ctx context.Context, token string, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	return verifyFormPost(ctx, hCaptchaVerifier.httpClient, hCaptchaVerifyURL, hCaptchaVerifier.secretKey, token, remoteIP)
}

func verifyFormPost(ctx context.Context, httpClient HTTPClient, endpointURL string, secretKey string, token string, remoteIP string) (bool, error) {
	formValues := url.Values{}
	formValues.Set("secret", secretKey)
	formValues.Set("response", token)
	if remoteIP != "" {
		formValues.Set("remoteip", remoteIP)
	}

	httpRequest, createErr := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, strings.NewReader(formValues.Encode()))
	if createErr != nil {
		return false, fmt.Errorf("failed to create captcha request: %w", createErr)
	}

	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	activeHTTPClient := httpClient
	if activeHTTPClient == nil {
		activeHTTPClient = &http.Client{Timeout: defaultCaptchaTimeout}
	}

	httpResponse, doErr := activeHTTPClient.Do(httpRequest)
	if doErr != nil {
		return false, fmt.Errorf("failed to execute captcha verification: %w", doErr)
	}

	defer func() {
		_ = httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode != http.StatusOK {
		return false, fmt.Errorf("captcha verification returned status %d", httpResponse.StatusCode)
	}

	var parsedResponse struct {
		Success bool `json:"success"`
	}
	if decodeErr := json.NewDecoder(httpResponse.Body).Decode(&parsedResponse); decodeErr != nil {
		return false, fmt.Errorf("failed to decode captcha response: %w", decodeErr)
	}

	return parsedResponse.Success, nil
}
