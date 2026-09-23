package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func newMissingScopeContext(ctx context.Context) context.Context {
	authContext := core.AuthContext{
		ServiceAccountID: "unauthorized-account",
		JWT: core.JWTClaims{
			Subject:  "unauthorized-account",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "unrelated:scope",
		},
	}
	return core.WithAuthContext(ctx, authContext)
}

func TestTasksControlPlaneHandlerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	tasksService := NewService(kernel)
	controlPlaneHandler := tasksService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	t.Run("scope check failures on all endpoints", func(t *testing.T) {
		t.Parallel()
		ctx := newMissingScopeContext(context.Background())

		// Jobs
		listJobsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs", nil)
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListJobs(responseRecorder, listJobsRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		createJobRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleCreateJob(responseRecorder, createJobRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		getJobRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetJob(responseRecorder, getJobRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		updateJobRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, updateJobRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		deleteJobRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleDeleteJob(responseRecorder, deleteJobRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// Executions
		triggerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleTriggerExecution(responseRecorder, triggerRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		listExecutionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/executions", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(responseRecorder, listExecutionsRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// DLQ
		listDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/dlq", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListDLQ(responseRecorder, listDLQRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		retryDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/dlq/abc/retry", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleRetryDLQ(responseRecorder, retryDLQRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		purgeDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/dlq/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handlePurgeDLQ(responseRecorder, purgeDLQRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// Config
		getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/config", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(responseRecorder, getConfigRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		updateConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/tasks/config", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(responseRecorder, updateConfigRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// Stats
		getStatsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/stats", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(responseRecorder, getStatsRequest)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("bad json and uuid validation errors", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Bad JSON on create
		badBodyReader := strings.NewReader("{invalid-json}")
		createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", badBodyReader)
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateJob(responseRecorder, createRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad UUID on get job
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs/not-a-uuid", nil)
		getRequest.SetPathValue("id", "not-a-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetJob(responseRecorder, getRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad UUID on update job
		updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/not-a-uuid", nil)
		updateRequest.SetPathValue("id", "not-a-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, updateRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad JSON on update job
		validUUID := "0191eb58-75c1-7cb2-b7b5-0c7f1a30282b"
		badUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/"+validUUID, strings.NewReader("{bad"))
		badUpdateRequest.SetPathValue("id", validUUID)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, badUpdateRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad UUID on delete job
		deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/jobs/bad", nil)
		deleteRequest.SetPathValue("id", "bad")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleDeleteJob(responseRecorder, deleteRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad JSON on trigger execution
		triggerBadRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", strings.NewReader("{bad"))
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleTriggerExecution(responseRecorder, triggerBadRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad job_id query param on list executions
		listExecutionsBadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/executions?job_id=bad-uuid", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(responseRecorder, listExecutionsBadRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad UUID on retry DLQ
		retryDLQBadRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/dlq/bad/retry", nil)
		retryDLQBadRequest.SetPathValue("id", "bad")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleRetryDLQ(responseRecorder, retryDLQBadRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad UUID on purge DLQ
		purgeDLQBadRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/dlq/bad", nil)
		purgeDLQBadRequest.SetPathValue("id", "bad")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handlePurgeDLQ(responseRecorder, purgeDLQBadRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Bad JSON on update config
		updateConfigBadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/tasks/config", strings.NewReader("{bad"))
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(responseRecorder, updateConfigBadRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("config endpoints get and update", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Get config succeeds
		getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/config", nil)
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(responseRecorder, getConfigRequest)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		// Update config with invalid validation returns 400
		badConfigJSON := `{"concurrency_limit": -1}`
		updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/tasks/config", strings.NewReader(badConfigJSON))
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(responseRecorder, updateRequest)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})
}
