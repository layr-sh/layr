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

func TestTasksControlPlaneHandlerJobsUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	tasksService := NewService(kernel)
	controlPlaneHandler := tasksService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	readJobAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksJobRead,
		},
	}
	readJobCtx := core.WithAuthContext(context.Background(), readJobAuthContext)

	writeJobAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeTasksJobWrite,
		},
	}
	writeJobCtx := core.WithAuthContext(context.Background(), writeJobAuthContext)

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
		// ListJobs without scope
		request := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/jobs", nil)
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListJobs(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// CreateJob without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/tasks/jobs", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleCreateJob(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// GetJob without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetJob(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// UpdateJob without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodPatch, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		// DeleteJob without scope
		request = httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/tasks/jobs/abc", nil)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleDeleteJob(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("input parsing errors", func(t *testing.T) {
		// CreateJob bad JSON
		request := httptest.NewRequestWithContext(writeJobCtx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte("{bad")))
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateJob(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// GetJob invalid UUID
		request = httptest.NewRequestWithContext(readJobCtx, http.MethodGet, "/v1/_/tasks/jobs/invalid-uuid", nil)
		request.SetPathValue("job_id", "invalid-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetJob(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// UpdateJob invalid UUID
		request = httptest.NewRequestWithContext(writeJobCtx, http.MethodPatch, "/v1/_/tasks/jobs/invalid-uuid", nil)
		request.SetPathValue("job_id", "invalid-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// UpdateJob bad JSON
		validID := "0191eb58-75c1-7cb2-b7b5-0c7f1a30282b"
		request = httptest.NewRequestWithContext(writeJobCtx, http.MethodPatch, "/v1/_/tasks/jobs/"+validID, bytes.NewReader([]byte("{bad")))
		request.SetPathValue("job_id", validID)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// DeleteJob invalid UUID
		request = httptest.NewRequestWithContext(writeJobCtx, http.MethodDelete, "/v1/_/tasks/jobs/invalid-uuid", nil)
		request.SetPathValue("job_id", "invalid-uuid")
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleDeleteJob(responseRecorder, request)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("database error handling", func(t *testing.T) {
		validUUID := "0191eb58-75c1-7cb2-b7b5-0c7f1a30282b"

		// ListJobs DB error -> 500
		listJobsRequest := httptest.NewRequestWithContext(readJobCtx, http.MethodGet, "/v1/_/tasks/jobs", nil)
		listJobsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListJobs(listJobsResponseRecorder, listJobsRequest)
		require.Equal(t, http.StatusInternalServerError, listJobsResponseRecorder.Code)

		// CreateJob DB error -> 400 (validation/creation error)
		createPayload := []byte(`{"name":"test-job","cron_expression":"* * * * *","target_type":"sql","target_payload":{"query":"SELECT 1"}}`)
		createJobRequest := httptest.NewRequestWithContext(writeJobCtx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader(createPayload))
		createJobResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateJob(createJobResponseRecorder, createJobRequest)
		require.Equal(t, http.StatusBadRequest, createJobResponseRecorder.Code)

		// GetJob DB error -> 500
		getJobRequest := httptest.NewRequestWithContext(readJobCtx, http.MethodGet, "/v1/_/tasks/jobs/"+validUUID, nil)
		getJobRequest.SetPathValue("job_id", validUUID)
		getJobResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetJob(getJobResponseRecorder, getJobRequest)
		require.Equal(t, http.StatusInternalServerError, getJobResponseRecorder.Code)

		// UpdateJob DB error -> 400
		updatePayload := []byte(`{"name":"updated-job"}`)
		updateJobRequest := httptest.NewRequestWithContext(writeJobCtx, http.MethodPatch, "/v1/_/tasks/jobs/"+validUUID, bytes.NewReader(updatePayload))
		updateJobRequest.SetPathValue("job_id", validUUID)
		updateJobResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateJob(updateJobResponseRecorder, updateJobRequest)
		require.Equal(t, http.StatusBadRequest, updateJobResponseRecorder.Code)

		// DeleteJob DB error -> 500
		deleteJobRequest := httptest.NewRequestWithContext(writeJobCtx, http.MethodDelete, "/v1/_/tasks/jobs/"+validUUID, nil)
		deleteJobRequest.SetPathValue("job_id", validUUID)
		deleteJobResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteJob(deleteJobResponseRecorder, deleteJobRequest)
		require.Equal(t, http.StatusInternalServerError, deleteJobResponseRecorder.Code)
	})
}
