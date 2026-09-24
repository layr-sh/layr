package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerJobsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	tasksService := NewService(kernel)
	require.NoError(t, tasksService.Start(ctx))
	defer tasksService.Stop()

	coreServer := core.NewServer(kernel)
	tasksService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "tasks-jobs-admin",
		Scopes: []string{
			core.ScopeTasksJobRead,
			core.ScopeTasksJobWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("jobs CRUD control plane API lifecycle", func(t *testing.T) {
		// 1. POST /v1/_/tasks/jobs
		createPayload := `{"name":"api-cron-job","cron_expression":"*/10 * * * *","target_type":"sql","target_payload":{"query":"SELECT 1"}}`
		postRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createPayload)))
		postRequest.Header.Set("Content-Type", "application/json")
		postRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		postResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(postResponseRecorder, postRequest)
		require.Equal(t, http.StatusCreated, postResponseRecorder.Code)

		var createdJob Job
		require.NoError(t, json.Unmarshal(postResponseRecorder.Body.Bytes(), &createdJob))
		require.Equal(t, "api-cron-job", createdJob.Name)

		// 2. GET /v1/_/tasks/jobs
		listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs", nil)
		listRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		listResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusOK, listResponseRecorder.Code)

		var listJobsResponse ListJobsResponse
		require.NoError(t, json.Unmarshal(listResponseRecorder.Body.Bytes(), &listJobsResponse))
		require.Equal(t, 1, listJobsResponse.Count)

		// 3. GET /v1/_/tasks/jobs/{id}
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs/"+createdJob.ID.String(), nil)
		getRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var getJobResponse GetJobResponse
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &getJobResponse))
		require.Equal(t, createdJob.ID, getJobResponse.ID)

		// 4. PATCH /v1/_/tasks/jobs/{id}
		updatePayload := `{"name":"api-cron-job-renamed"}`
		patchRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/"+createdJob.ID.String(), bytes.NewReader([]byte(updatePayload)))
		patchRequest.Header.Set("Content-Type", "application/json")
		patchRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		patchResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(patchResponseRecorder, patchRequest)
		require.Equal(t, http.StatusOK, patchResponseRecorder.Code)

		// 5. DELETE /v1/_/tasks/jobs/{id}
		deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/jobs/"+createdJob.ID.String(), nil)
		deleteRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		deleteResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)

		// 6. GET after DELETE -> 404
		getAfterDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/jobs/"+createdJob.ID.String(), nil)
		getAfterDeleteRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getAfterDeleteResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getAfterDeleteResponseRecorder, getAfterDeleteRequest)
		require.Equal(t, http.StatusNotFound, getAfterDeleteResponseRecorder.Code)

		// 7. Duplicate job name -> 409 Conflict
		dupPostRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createPayload)))
		dupPostRequest.Header.Set("Content-Type", "application/json")
		dupPostRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		dupPostResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(dupPostResponseRecorder, dupPostRequest)
		require.Equal(t, http.StatusCreated, dupPostResponseRecorder.Code)

		duplicatePostRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createPayload)))
		duplicatePostRequest.Header.Set("Content-Type", "application/json")
		duplicatePostRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		duplicatePostResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(duplicatePostResponseRecorder, duplicatePostRequest)
		require.Equal(t, http.StatusConflict, duplicatePostResponseRecorder.Code)

		// 8. Update non-existent job -> 404 Not Found
		randomUUID := "0191eb58-75c1-7cb2-b7b5-0c7f1a30282b"
		patchNotFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/"+randomUUID, bytes.NewReader([]byte(`{"name":"random"}`)))
		patchNotFoundRequest.Header.Set("Content-Type", "application/json")
		patchNotFoundRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		patchNotFoundResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(patchNotFoundResponseRecorder, patchNotFoundRequest)
		require.Equal(t, http.StatusNotFound, patchNotFoundResponseRecorder.Code)

		// 9. Update job name to existing duplicate name -> 409 Conflict
		createSecondPayload := `{"name":"second-job","cron_expression":"*/10 * * * *","target_type":"sql","target_payload":{"query":"SELECT 1"}}`
		secondPostRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/tasks/jobs", bytes.NewReader([]byte(createSecondPayload)))
		secondPostRequest.Header.Set("Content-Type", "application/json")
		secondPostRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		secondPostResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(secondPostResponseRecorder, secondPostRequest)
		require.Equal(t, http.StatusCreated, secondPostResponseRecorder.Code)

		var secondJob Job
		require.NoError(t, json.Unmarshal(secondPostResponseRecorder.Body.Bytes(), &secondJob))

		patchConflictRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/_/tasks/jobs/"+secondJob.ID.String(), bytes.NewReader([]byte(`{"name":"api-cron-job"}`)))
		patchConflictRequest.Header.Set("Content-Type", "application/json")
		patchConflictRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		patchConflictResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(patchConflictResponseRecorder, patchConflictRequest)
		require.Equal(t, http.StatusConflict, patchConflictResponseRecorder.Code)

		// 10. Delete non-existent job -> 404 Not Found
		deleteNotFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/tasks/jobs/"+randomUUID, nil)
		deleteNotFoundRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		deleteNotFoundResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(deleteNotFoundResponseRecorder, deleteNotFoundRequest)
		require.Equal(t, http.StatusNotFound, deleteNotFoundResponseRecorder.Code)
	})
}
