package tasks

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerExecutionsUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	tasksService := NewService(kernel)
	controlPlaneHandler := tasksService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	readAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksExecutionRead,
		},
	}
	readCtx := core.WithAuthContext(context.Background(), readAuthContext)

	writeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksExecutionWrite,
		},
	}
	writeCtx := core.WithAuthContext(context.Background(), writeAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(context.Background(), noScopeAuthContext)

	t.Run("scope check rejections", func(t *testing.T) {
		// TriggerExecution without scope
		request := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/tasks/executions", nil)
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleTriggerExecution(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// ListExecutions without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/executions", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// ListDLQ without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/dlq", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListDLQ(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// RetryDLQ without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/tasks/dlq/abc/retry", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleRetryDLQ(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// PurgeDLQ without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/tasks/dlq/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handlePurgeDLQ(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("input parsing errors", func(t *testing.T) {
		// TriggerExecution bad JSON
		request := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte("{bad")))
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleTriggerExecution(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// ListExecutions bad job_id query param
		request = httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/tasks/executions?job_id=not-a-uuid", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// RetryDLQ bad UUID
		request = httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/tasks/dlq/invalid-uuid/retry", nil)
		request.SetPathValue("id", "invalid-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleRetryDLQ(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// PurgeDLQ bad UUID
		request = httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/tasks/dlq/invalid-uuid", nil)
		request.SetPathValue("id", "invalid-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handlePurgeDLQ(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})
}

func TestTasksControlPlaneHandlerExecutionsDatabaseErrorsUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenTasksService := NewService(brokenKernel)
	brokenControlPlaneHandler := brokenTasksService.ControlPlaneHandler()
	randomUUID := "0191eb58-75c1-7cb2-b7b5-0c7f1a30282b"

	readAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksExecutionRead,
		},
	}
	readCtx := core.WithAuthContext(context.Background(), readAuthContext)

	writeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksExecutionWrite,
		},
	}
	writeCtx := core.WithAuthContext(context.Background(), writeAuthContext)

	// TriggerExecution broken DB error -> 400
	triggerNotFoundBody := []byte(`{"job_id":"` + randomUUID + `"}`)
	request := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader(triggerNotFoundBody))
	responseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleTriggerExecution(responseRecorder, request)
	require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

	// ListExecutions broken DB -> 500
	request = httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/tasks/executions", nil)
	responseRecorder = httptest.NewRecorder()
	brokenControlPlaneHandler.handleListExecutions(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

	// ListDLQ broken DB -> 500
	request = httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/tasks/dlq", nil)
	responseRecorder = httptest.NewRecorder()
	brokenControlPlaneHandler.handleListDLQ(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

	// RetryDLQ broken DB -> 500
	request = httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/tasks/dlq/"+randomUUID+"/retry", nil)
	request.SetPathValue("id", randomUUID)
	responseRecorder = httptest.NewRecorder()
	brokenControlPlaneHandler.handleRetryDLQ(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

	// PurgeDLQ broken DB -> 500
	request = httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/tasks/dlq/"+randomUUID, nil)
	request.SetPathValue("id", randomUUID)
	responseRecorder = httptest.NewRecorder()
	brokenControlPlaneHandler.handlePurgeDLQ(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
}
