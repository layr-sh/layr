package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerExecutionsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	tasksService := NewService(kernel)
	require.NoError(t, tasksService.Start(ctx))
	defer tasksService.Stop()

	coreServer := core.NewServer(kernel)
	tasksService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "tasks-executions-admin",
		Scopes: []string{
			core.ScopeTasksJobRead,
			core.ScopeTasksJobWrite,
			core.ScopeTasksExecutionRead,
			core.ScopeTasksExecutionWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("executions and dlq control plane API lifecycle", func(t *testing.T) {
		// 1. Create a job first
		createPayload := `{"name":"exec-job","cron_expression":"0 0 1 1 *","target_type":"sql","target_payload":{"query":"SELECT 1"}}`
		postJobRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createPayload)))
		postJobRequest.Header.Set("Content-Type", "application/json")
		postJobRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		postJobResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(postJobResponseRecorder, postJobRequest)
		require.Equal(t, http.StatusCreated, postJobResponseRecorder.Code)

		var createdJob Job
		require.NoError(t, json.Unmarshal(postJobResponseRecorder.Body.Bytes(), &createdJob))

		// 2. POST /v1/_/tasks/executions (Trigger execution)
		triggerPayload := `{"job_id":"` + createdJob.ID.String() + `"}`
		triggerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte(triggerPayload)))
		triggerRequest.Header.Set("Content-Type", "application/json")
		triggerRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		triggerResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(triggerResponseRecorder, triggerRequest)
		require.Equal(t, http.StatusAccepted, triggerResponseRecorder.Code)

		var triggerExecutionResponse TriggerExecutionResponse
		require.NoError(t, json.Unmarshal(triggerResponseRecorder.Body.Bytes(), &triggerExecutionResponse))
		require.Equal(t, "pending", triggerExecutionResponse.Status)

		// 3. GET /v1/_/tasks/executions
		listExecutionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/executions?limit=10&offset=5", nil)
		listExecutionsRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		listExecutionsResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listExecutionsResponseRecorder, listExecutionsRequest)
		require.Equal(t, http.StatusOK, listExecutionsResponseRecorder.Code)

		// 4. Force execution to failed status in DB to test DLQ endpoints
		_, err := kernel.DB().Exec(ctx, "UPDATE tasks.executions SET status = 'failed' WHERE id = $1", triggerExecutionResponse.ExecutionID)
		require.NoError(t, err)

		// 5. GET /v1/_/tasks/dlq
		listDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/dlq?limit=10&offset=5", nil)
		listDLQRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		listDLQResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listDLQResponseRecorder, listDLQRequest)
		require.Equal(t, http.StatusOK, listDLQResponseRecorder.Code)

		var listDLQResponse ListDLQResponse
		require.NoError(t, json.Unmarshal(listDLQResponseRecorder.Body.Bytes(), &listDLQResponse))
		require.Positive(t, listDLQResponse.Count)

		// 6. POST /v1/_/tasks/dlq/{id}/retry
		retryDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/dlq/"+triggerExecutionResponse.ExecutionID.String()+"/retry", nil)
		retryDLQRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		retryDLQResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(retryDLQResponseRecorder, retryDLQRequest)
		require.Equal(t, http.StatusOK, retryDLQResponseRecorder.Code)

		// 7. Force back to failed and test DELETE /v1/_/tasks/dlq/{id}
		_, err = kernel.DB().Exec(ctx, "UPDATE tasks.executions SET status = 'failed' WHERE id = $1", triggerExecutionResponse.ExecutionID)
		require.NoError(t, err)

		purgeDLQRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/dlq/"+triggerExecutionResponse.ExecutionID.String(), nil)
		purgeDLQRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		purgeDLQResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(purgeDLQResponseRecorder, purgeDLQRequest)
		require.Equal(t, http.StatusNoContent, purgeDLQResponseRecorder.Code)

		// 8. Retry non-existent DLQ returns 404
		badRetryRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/dlq/"+uuid.NewV7().String()+"/retry", nil)
		badRetryRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		badRetryResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(badRetryResponseRecorder, badRetryRequest)
		require.Equal(t, http.StatusNotFound, badRetryResponseRecorder.Code)

		// 9. Delete non-existent DLQ returns 404
		badPurgeRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/dlq/"+uuid.NewV7().String(), nil)
		badPurgeRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		badPurgeResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(badPurgeResponseRecorder, badPurgeRequest)
		require.Equal(t, http.StatusNotFound, badPurgeResponseRecorder.Code)

		// 10. Trigger non-existent job returns 404
		badTriggerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte(`{"job_id":"`+uuid.NewV7().String()+`"}`)))
		badTriggerRequest.Header.Set("Content-Type", "application/json")
		badTriggerRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		badTriggerResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(badTriggerResponseRecorder, badTriggerRequest)
		require.Equal(t, http.StatusNotFound, badTriggerResponseRecorder.Code)

		// 11. Trigger with invalid JSON body returns 400
		invalidTriggerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/executions", bytes.NewReader([]byte(`{"job_id":"not-a-valid-uuid"}`)))
		invalidTriggerRequest.Header.Set("Content-Type", "application/json")
		invalidTriggerRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		invalidTriggerResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(invalidTriggerResponseRecorder, invalidTriggerRequest)
		require.Equal(t, http.StatusBadRequest, invalidTriggerResponseRecorder.Code)
	})
}
