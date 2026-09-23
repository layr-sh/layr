// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksFullLifecycleE2E(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Tasks Service and start background loops
	tasksService := NewService(kernel)
	require.NoError(t, tasksService.Start(ctx))
	defer tasksService.Stop()

	coreServer := core.NewServer(kernel)
	tasksService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// 2. Create Service Account with full tasks scopes
	createdServiceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "tasks-e2e-service-account",
		Scopes: []string{
			core.ScopeTasksJobRead,
			core.ScopeTasksJobWrite,
			core.ScopeTasksExecutionRead,
			core.ScopeTasksExecutionWrite,
			core.ScopeTasksConfigRead,
			core.ScopeTasksConfigWrite,
			core.ScopeTasksStatsRead,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := createdServiceAccount.SecretKey

	// 3. Control Plane: Inspect and update runtime configuration
	getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/config", nil)
	getConfigRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	getConfigResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getConfigResponseRecorder, getConfigRequest)
	require.Equal(t, http.StatusOK, getConfigResponseRecorder.Code)

	var activeConfig Config
	require.NoError(t, json.Unmarshal(getConfigResponseRecorder.Body.Bytes(), &activeConfig))
	require.Positive(t, activeConfig.PollIntervalMs)

	// Update configuration
	activeConfig.PollIntervalMs = 500
	updateConfigBytes, _ := json.Marshal(activeConfig)
	updateConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/tasks/config", bytes.NewReader(updateConfigBytes))
	updateConfigRequest.Header.Set("Content-Type", "application/json")
	updateConfigRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	updateConfigResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(updateConfigResponseRecorder, updateConfigRequest)
	require.Equal(t, http.StatusOK, updateConfigResponseRecorder.Code)

	// 4. Control Plane: Create a recurring SQL Job
	createJobPayload := `{"name":"e2e-recurring-sql","cron_expression":"* * * * *","target_type":"sql","target_payload":{"query":"SELECT 1"}}`
	createJobRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createJobPayload)))
	createJobRequest.Header.Set("Content-Type", "application/json")
	createJobRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	createJobResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createJobResponseRecorder, createJobRequest)
	require.Equal(t, http.StatusCreated, createJobResponseRecorder.Code)

	var createdJob Job
	require.NoError(t, json.Unmarshal(createJobResponseRecorder.Body.Bytes(), &createdJob))
	require.Equal(t, "e2e-recurring-sql", createdJob.Name)

	// 5. Control Plane: List Jobs
	listJobsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs", nil)
	listJobsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	listJobsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listJobsResponseRecorder, listJobsRequest)
	require.Equal(t, http.StatusOK, listJobsResponseRecorder.Code)

	var listJobsResponse ListJobsResponse
	require.NoError(t, json.Unmarshal(listJobsResponseRecorder.Body.Bytes(), &listJobsResponse))
	require.GreaterOrEqual(t, listJobsResponse.Count, 1)

	// 6. Control Plane: Get Specific Job by ID
	getJobRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/v1/_/tasks/jobs/%s", createdJob.ID), nil)
	getJobRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	getJobResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getJobResponseRecorder, getJobRequest)
	require.Equal(t, http.StatusOK, getJobResponseRecorder.Code)

	// 7. Control Plane: Trigger Immediate Execution
	triggerPayload := fmt.Sprintf(`{"job_id":"%s"}`, createdJob.ID)
	triggerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte(triggerPayload)))
	triggerRequest.Header.Set("Content-Type", "application/json")
	triggerRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	triggerResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(triggerResponseRecorder, triggerRequest)
	require.Equal(t, http.StatusAccepted, triggerResponseRecorder.Code)

	var triggerExecutionResponse TriggerExecutionResponse
	require.NoError(t, json.Unmarshal(triggerResponseRecorder.Body.Bytes(), &triggerExecutionResponse))
	require.NotEmpty(t, triggerExecutionResponse.ExecutionID)

	// 8. Wait for Worker to process execution
	require.Eventually(t, func() bool {
		var logCount int
		scanErr := kernel.DB().QueryRow(ctx, "SELECT count(*) FROM tasks.execution_logs WHERE execution_id = $1", triggerExecutionResponse.ExecutionID).Scan(&logCount)
		return scanErr == nil && logCount > 0
	}, 10*time.Second, 100*time.Millisecond)

	// 9. Control Plane: List Execution Logs
	listExecutionsURL := fmt.Sprintf("/v1/_/tasks/executions?job_id=%s", createdJob.ID)
	listExecutionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, listExecutionsURL, nil)
	listExecutionsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	listExecutionsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listExecutionsResponseRecorder, listExecutionsRequest)
	require.Equal(t, http.StatusOK, listExecutionsResponseRecorder.Code)

	// 10. DLQ Flow: Trigger HTTP task pointing to a failing endpoint (400 Client Error -> immediate DLQ)
	failingWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "bad request from test", http.StatusBadRequest)
	}))
	defer failingWebhookServer.Close()

	createFailingJobPayload := fmt.Sprintf(`{"name":"e2e-dlq-job","cron_expression":"* * * * *","target_type":"http","target_payload":{"url":"%s"}}`, failingWebhookServer.URL)
	createFailingJobRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createFailingJobPayload)))
	createFailingJobRequest.Header.Set("Content-Type", "application/json")
	createFailingJobRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	createFailingJobResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createFailingJobResponseRecorder, createFailingJobRequest)
	require.Equal(t, http.StatusCreated, createFailingJobResponseRecorder.Code)

	var failingJob Job
	require.NoError(t, json.Unmarshal(createFailingJobResponseRecorder.Body.Bytes(), &failingJob))

	triggerDLQPayload := fmt.Sprintf(`{"job_id":"%s"}`, failingJob.ID)
	triggerDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte(triggerDLQPayload)))
	triggerDLQRequest.Header.Set("Content-Type", "application/json")
	triggerDLQRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	triggerDLQResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(triggerDLQResponseRecorder, triggerDLQRequest)
	require.Equal(t, http.StatusAccepted, triggerDLQResponseRecorder.Code)

	var dlqTriggerExecutionResponse TriggerExecutionResponse
	require.NoError(t, json.Unmarshal(triggerDLQResponseRecorder.Body.Bytes(), &dlqTriggerExecutionResponse))

	// Wait for DLQ entry (status = 'failed')
	require.Eventually(t, func() bool {
		var status string
		scanErr := kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", dlqTriggerExecutionResponse.ExecutionID).Scan(&status)
		return scanErr == nil && status == StatusFailed
	}, 10*time.Second, 100*time.Millisecond)

	// 11. Control Plane: List Dead-Letter Queue
	getDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/dlq", nil)
	getDLQRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	getDLQResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(getDLQResponseRecorder, getDLQRequest)
	require.Equal(t, http.StatusOK, getDLQResponseRecorder.Code)

	// 12. Control Plane: Redrive execution from DLQ
	redriveURL := fmt.Sprintf("/v1/_/tasks/dlq/%s/retry", dlqTriggerExecutionResponse.ExecutionID)
	redriveRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, redriveURL, nil)
	redriveRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	redriveResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(redriveResponseRecorder, redriveRequest)
	require.Equal(t, http.StatusOK, redriveResponseRecorder.Code)

	// Update to failed to test deletion/purge
	_, _ = kernel.DB().Exec(ctx, "UPDATE tasks.executions SET status = 'failed' WHERE id = $1", dlqTriggerExecutionResponse.ExecutionID)

	// 13. Control Plane: Purge/Delete execution from DLQ
	purgeURL := fmt.Sprintf("/v1/_/tasks/dlq/%s", dlqTriggerExecutionResponse.ExecutionID)
	purgeRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, purgeURL, nil)
	purgeRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	purgeResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(purgeResponseRecorder, purgeRequest)
	require.Equal(t, http.StatusNoContent, purgeResponseRecorder.Code)

	// 14. Control Plane: Query Stats
	statsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/stats", nil)
	statsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	statsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusOK, statsResponseRecorder.Code)

	var statsResponse StatsResponse
	require.NoError(t, json.Unmarshal(statsResponseRecorder.Body.Bytes(), &statsResponse))
	require.GreaterOrEqual(t, statsResponse.TotalJobs, 1)

	// 15. Control Plane: Update Job to paused, then delete
	pausePayload := `{"is_enabled":false}`
	pauseRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("/v1/_/tasks/jobs/%s", createdJob.ID), bytes.NewReader([]byte(pausePayload)))
	pauseRequest.Header.Set("Content-Type", "application/json")
	pauseRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	pauseResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(pauseResponseRecorder, pauseRequest)
	require.Equal(t, http.StatusOK, pauseResponseRecorder.Code)

	deleteJobRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("/v1/_/tasks/jobs/%s", createdJob.ID), nil)
	deleteJobRequest.Header.Set("X-Service-Account-Key", authSecretKey)
	deleteJobResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteJobResponseRecorder, deleteJobRequest)
	require.Equal(t, http.StatusNoContent, deleteJobResponseRecorder.Code)
}
