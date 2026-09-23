// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
)

const (
	maxLoggedResponseBodyBytes = 1024
	defaultDispatcherTimeout   = 30
	maxBackoffExponentShift    = 30
)

// Dispatcher executes HTTP and SQL tasks mirroring the core event hook architecture.
type Dispatcher struct {
	kernel     *core.Kernel
	httpClient *http.Client
}

// NewDispatcher initializes a new Dispatcher.
func NewDispatcher(kernel *core.Kernel) *Dispatcher {
	return &Dispatcher{
		kernel:     kernel,
		httpClient: &http.Client{},
	}
}

// SetHTTPClient overrides the HTTP client (useful for unit/integration tests).
func (dispatcher *Dispatcher) SetHTTPClient(httpClient *http.Client) {
	dispatcher.httpClient = httpClient
}

// ExecuteHTTPAttempt dispatches an HTTP POST webhook request with SSRF protection and HMAC signature.
func (dispatcher *Dispatcher) ExecuteHTTPAttempt(ctx context.Context, targetURL string, headers map[string]string, bodyBytes []byte, jobID *uuid.UUID, executionID uuid.UUID, timeoutSeconds int) (*int, *string, bool, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultDispatcherTimeout
	}

	if err := validateWebhookTargetURL(targetURL); err != nil {
		dispatcher.kernel.EventBus().Publish(ctx, NewThreatSSRFBlockedEvent(targetURL, ThreatSSRFBlockedEventData{
			TargetURL: targetURL,
			Reason:    err.Error(),
		}))
		return nil, nil, true, fmt.Errorf("ssrf blocked: %w", err)
	}

	attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	request, _ := http.NewRequestWithContext(attemptCtx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))

	signingKey := dispatcher.kernel.CryptoKeyManager().DeriveSubkey(core.CryptoContextTasksWebhookHMAC)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := ComputeWebhookSignature(signingKey, timestamp, bodyBytes)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Layr-Tasks/1.0")
	request.Header.Set("X-Layr-Signature", signature)
	request.Header.Set("X-Layr-Timestamp", timestamp)
	if jobID != nil {
		request.Header.Set("X-Layr-Job-ID", jobID.String())
	}
	request.Header.Set("X-Layr-Execution-ID", executionID.String())

	for headerKey, headerValue := range headers {
		request.Header.Set(headerKey, headerValue)
	}

	response, err := dispatcher.httpClient.Do(request)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to execute http request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	statusCode := response.StatusCode
	var responseBuffer bytes.Buffer
	_, _ = responseBuffer.ReadFrom(response.Body)
	responseText := responseBuffer.String()
	if len(responseText) > maxLoggedResponseBodyBytes {
		responseText = responseText[:maxLoggedResponseBodyBytes]
	}

	isClientError := (statusCode >= 400 && statusCode < 500 && statusCode != http.StatusTooManyRequests)
	return &statusCode, &responseText, isClientError, nil
}

// ExecuteSQLAttempt executes a direct SQL query or stored procedure against PostgreSQL.
func (dispatcher *Dispatcher) ExecuteSQLAttempt(ctx context.Context, query string, params []any, timeoutSeconds int) (int64, *string, bool, error) {
	if strings.TrimSpace(query) == "" {
		return 0, nil, false, errors.New("sql query is empty")
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultDispatcherTimeout
	}

	attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	execResult, err := dispatcher.kernel.DB().Exec(attemptCtx, query, params...)

	if err != nil {
		isTransient := isTransientDatabaseError(err)
		return 0, nil, isTransient, err
	}

	rowsAffected := execResult.RowsAffected()
	resultSummary := fmt.Sprintf("%d rows affected", rowsAffected)
	return rowsAffected, &resultSummary, false, nil
}

// ComputeWebhookSignature computes HMAC-SHA256 signature for payload verification.
func ComputeWebhookSignature(secret []byte, timestamp string, payload []byte) string {
	message := fmt.Sprintf("%s.%s", timestamp, string(payload))
	hmacHash := hmac.New(sha256.New, secret)
	hmacHash.Write([]byte(message))
	return hex.EncodeToString(hmacHash.Sum(nil))
}

// VerifyWebhookSignature verifies an incoming HMAC-SHA256 signature.
func VerifyWebhookSignature(secret []byte, signature, timestamp string, payload []byte) bool {
	expected := ComputeWebhookSignature(secret, timestamp, payload)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func validateWebhookTargetURL(targetURL string) error {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid webhook target url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook scheme must be http or https")
	}
	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		return fmt.Errorf("webhook target hostname is required")
	}
	if hostname == "metadata.google.internal" || hostname == "metadata" {
		return fmt.Errorf("webhook target to metadata service is prohibited")
	}
	if parsedIP := net.ParseIP(hostname); parsedIP != nil {
		if parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() {
			return fmt.Errorf("webhook target to link-local address is prohibited: %s", hostname)
		}
	}
	return nil
}

func isTransientDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		if pgError.Code == "40001" || pgError.Code == "40P01" || pgError.Code == "57014" || strings.HasPrefix(pgError.Code, "08") {
			return true
		}
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "40001") ||
		strings.Contains(errMsg, "40p01") ||
		strings.Contains(errMsg, "57014") ||
		strings.Contains(errMsg, "deadlock") ||
		strings.Contains(errMsg, "serialization failure") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "server closed the connection")
}

// CalculateTaskBackoff computes exponential backoff with random jitter.
func CalculateTaskBackoff(attempt int, initialDelaySeconds, maxDelaySeconds int) time.Duration {
	if initialDelaySeconds <= 0 {
		initialDelaySeconds = 1
	}
	if maxDelaySeconds < initialDelaySeconds {
		maxDelaySeconds = initialDelaySeconds
	}

	shift := attempt
	if shift > maxBackoffExponentShift {
		shift = maxBackoffExponentShift
	}

	delaySeconds := int64(initialDelaySeconds) * (1 << shift)
	if delaySeconds > int64(maxDelaySeconds) {
		delaySeconds = int64(maxDelaySeconds)
	}

	// Add random jitter of up to 25% of delay
	maxJitterMs := (delaySeconds * 1000) / 4
	var jitterMs int64
	if maxJitterMs > 0 {
		n, randErr := rand.Int(rand.Reader, big.NewInt(maxJitterMs))
		if randErr == nil {
			jitterMs = n.Int64()
		}
	}

	return time.Duration(delaySeconds)*time.Second + time.Duration(jitterMs)*time.Millisecond
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
