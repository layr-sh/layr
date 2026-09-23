package tasks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksDispatcherUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	dispatcher := NewDispatcher(kernel)
	require.NotNil(t, dispatcher)
	dispatcher.SetHTTPClient(&http.Client{})

	testExecutionID := uuid.NewV7()
	testJobID := uuid.NewV7()

	t.Run("calculates task backoff with bounds and jitter", func(t *testing.T) {
		t.Parallel()
		backoffDuration := CalculateTaskBackoff(0, 0, 0)
		require.Positive(t, backoffDuration)

		backoffDurationAttempt1 := CalculateTaskBackoff(1, 2, 100)
		require.GreaterOrEqual(t, backoffDurationAttempt1, 4*time.Second)

		backoffDurationAttemptLarge := CalculateTaskBackoff(35, 1, 30)
		require.LessOrEqual(t, backoffDurationAttemptLarge, 40*time.Second)
	})

	t.Run("sleep with context respects cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		completed := sleepWithContext(ctx, 5*time.Second)
		require.False(t, completed)

		completedNormal := sleepWithContext(context.Background(), time.Millisecond)
		require.True(t, completedNormal)
	})

	t.Run("validates webhook target url and blocks ssrf targets", func(t *testing.T) {
		t.Parallel()
		require.Error(t, validateWebhookTargetURL("://invalid-url"))
		require.Error(t, validateWebhookTargetURL("ftp://example.com/webhook"))
		require.Error(t, validateWebhookTargetURL("http:///no-host"))
		require.Error(t, validateWebhookTargetURL("http://metadata.google.internal/computeMetadata/v1"))
		require.Error(t, validateWebhookTargetURL("http://metadata/computeMetadata/v1"))
		require.Error(t, validateWebhookTargetURL("http://169.254.169.254/hook"))
		require.NoError(t, validateWebhookTargetURL("http://127.0.0.1:8080/hook"))
		require.NoError(t, validateWebhookTargetURL("https://api.example.com/webhook"))
		require.NoError(t, validateWebhookTargetURL("http://api.example.com/webhook"))
	})

	t.Run("detects transient database errors", func(t *testing.T) {
		t.Parallel()
		require.False(t, isTransientDatabaseError(nil))

		serializationPgError := &pgconn.PgError{Code: "40001"}
		require.True(t, isTransientDatabaseError(serializationPgError))

		deadlockPgError := &pgconn.PgError{Code: "40P01"}
		require.True(t, isTransientDatabaseError(deadlockPgError))

		connExceptionPgError := &pgconn.PgError{Code: "08006"}
		require.True(t, isTransientDatabaseError(connExceptionPgError))

		syntaxPgError := &pgconn.PgError{Code: "42601"}
		require.False(t, isTransientDatabaseError(syntaxPgError))
	})

	t.Run("computes deterministic HMAC-SHA256 signature", func(t *testing.T) {
		t.Parallel()
		secret := []byte("super-secret")
		body := []byte(`{"event":"test"}`)
		timestamp := "1234567890"

		signature1 := ComputeWebhookSignature(secret, timestamp, body)
		signature2 := ComputeWebhookSignature(secret, timestamp, body)
		require.Equal(t, signature1, signature2)
		require.NotEmpty(t, signature1)
		require.True(t, VerifyWebhookSignature(secret, signature1, timestamp, body))
		require.False(t, VerifyWebhookSignature(secret, "bad-signature", timestamp, body))
	})

	t.Run("execute http blocks ssrf private target url", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, "http://169.254.169.254/hook", nil, nil, &testJobID, testExecutionID, 10,
		)
		require.Error(t, err)
		require.True(t, isClientError)
		require.Nil(t, statusCode)
		require.Nil(t, responseBody)
		require.Contains(t, err.Error(), "prohibited")
	})

	t.Run("execute http successful webhook call", func(t *testing.T) {
		t.Parallel()
		receivedHeader := ""
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			receivedHeader = request.Header.Get("X-Layr-Signature")
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr != nil {
				http.Error(responseWriter, readErr.Error(), http.StatusInternalServerError)
				return
			}
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(bodyBytes)
		}))
		defer testServer.Close()

		ctx := context.Background()
		customHeaders := map[string]string{"X-Custom": "test-header"}
		bodyBytes := []byte(`{"action":"sync"}`)

		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, testServer.URL, customHeaders, bodyBytes, &testJobID, testExecutionID, 10,
		)
		require.NoError(t, err)
		require.False(t, isClientError)
		require.NotNil(t, statusCode)
		require.Equal(t, http.StatusOK, *statusCode)
		require.NotNil(t, responseBody)
		require.NotEmpty(t, receivedHeader)
	})

	t.Run("execute http detects 4xx client errors", func(t *testing.T) {
		t.Parallel()
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		}))
		defer testServer.Close()

		ctx := context.Background()
		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, testServer.URL, nil, nil, nil, testExecutionID, 10,
		)
		require.NoError(t, err)
		require.True(t, isClientError)
		require.NotNil(t, statusCode)
		require.Equal(t, http.StatusUnauthorized, *statusCode)
		require.NotNil(t, responseBody)
	})

	t.Run("execute http detects 5xx server errors as non-client", func(t *testing.T) {
		t.Parallel()
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, "internal error", http.StatusInternalServerError)
		}))
		defer testServer.Close()

		ctx := context.Background()
		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, testServer.URL, nil, nil, nil, testExecutionID, 10,
		)
		require.NoError(t, err)
		require.False(t, isClientError)
		require.NotNil(t, statusCode)
		require.Equal(t, http.StatusInternalServerError, *statusCode)
		require.NotNil(t, responseBody)
	})

	t.Run("execute sql rejects empty sql", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		rowsAffected, resultSummary, isTransient, err := dispatcher.ExecuteSQLAttempt(ctx, "   ", nil, 10)
		require.Error(t, err)
		require.False(t, isTransient)
		require.Equal(t, int64(0), rowsAffected)
		require.Nil(t, resultSummary)
		require.Contains(t, err.Error(), "sql query is empty")
	})

	t.Run("execute http truncates large response body and handles zero timeout", func(t *testing.T) {
		t.Parallel()
		largeText := strings.Repeat("x", 2048)
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(largeText))
		}))
		defer testServer.Close()

		ctx := context.Background()
		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, testServer.URL, nil, nil, nil, testExecutionID, 0,
		)
		require.NoError(t, err)
		require.False(t, isClientError)
		require.NotNil(t, statusCode)
		require.Equal(t, http.StatusOK, *statusCode)
		require.NotNil(t, responseBody)
		require.Equal(t, 1024, len(*responseBody))
	})

	t.Run("execute sql returns error on broken db", func(t *testing.T) {
		t.Parallel()
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		brokenDispatcher := NewDispatcher(brokenKernel)

		ctx := context.Background()

		rowsAffected, resultSummary, isTransient, err := brokenDispatcher.ExecuteSQLAttempt(ctx, "SELECT 1", nil, 0)
		require.Error(t, err)
		require.False(t, isTransient)
		require.Equal(t, int64(0), rowsAffected)
		require.Nil(t, resultSummary)
	})
}
