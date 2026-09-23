package tasks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksDispatcherIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	dispatcher := NewDispatcher(kernel)
	require.NotNil(t, dispatcher)

	testExecutionID := uuid.NewV7()
	testJobID := uuid.NewV7()

	t.Run("ExecuteSQLAttempt executes successfully against postgres", func(t *testing.T) {
		const createSQL = `CREATE TABLE IF NOT EXISTS test_sql_tasks (id serial primary key, val text)`
		rowsAffected, resultSummary, isTransient, err := dispatcher.ExecuteSQLAttempt(ctx, createSQL, nil, 10)
		require.NoError(t, err)
		require.False(t, isTransient)
		require.NotNil(t, resultSummary)
		require.Zero(t, rowsAffected)

		const insertSQL = `INSERT INTO test_sql_tasks (val) VALUES ($1)`
		rowsAffected, resultSummary, isTransient, err = dispatcher.ExecuteSQLAttempt(ctx, insertSQL, []any{"hello"}, 10)
		require.NoError(t, err)
		require.False(t, isTransient)
		require.NotNil(t, resultSummary)
		require.Equal(t, int64(1), rowsAffected)
	})

	t.Run("ExecuteSQLAttempt returns error on SQL syntax error", func(t *testing.T) {
		const badSQL = `SELECT * FROM non_existent_table_12345`
		rowsAffected, resultSummary, isTransient, err := dispatcher.ExecuteSQLAttempt(ctx, badSQL, nil, 10)
		require.Error(t, err)
		require.False(t, isTransient)
		require.Zero(t, rowsAffected)
		require.Nil(t, resultSummary)
	})

	t.Run("ExecuteHTTPAttempt executes against local HTTP server", func(t *testing.T) {
		capturedHeader := ""
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			capturedHeader = request.Header.Get("X-Layr-Signature")
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr != nil {
				http.Error(responseWriter, readErr.Error(), http.StatusInternalServerError)
				return
			}
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(bodyBytes)
		}))
		defer testServer.Close()

		headers := map[string]string{"X-Test-Header": "value"}
		bodyBytes := []byte(`{"message":"ping"}`)

		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, testServer.URL, headers, bodyBytes, &testJobID, testExecutionID, 10,
		)
		require.NoError(t, err)
		require.False(t, isClientError)
		require.NotNil(t, statusCode)
		require.Equal(t, http.StatusOK, *statusCode)
		require.NotNil(t, responseBody)
		require.NotEmpty(t, capturedHeader)
	})

	t.Run("ExecuteHTTPAttempt returns error on unreachable network address", func(t *testing.T) {
		statusCode, responseBody, isClientError, err := dispatcher.ExecuteHTTPAttempt(
			ctx, "http://127.0.0.1:54321/no-listener", nil, nil, nil, testExecutionID, 1,
		)
		require.Error(t, err)
		require.False(t, isClientError)
		require.Nil(t, statusCode)
		require.Nil(t, responseBody)
	})
}
