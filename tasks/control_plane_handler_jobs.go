// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"encoding/json"
	"errors"
	"net/http"

	"uuid"

	"layr.sh/core"
)

// handleListJobs handles GET /v1/_/tasks/jobs returning all configured recurring cron jobs.
func (controlPlaneHandler *ControlPlaneHandler) handleListJobs(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleListJobs invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksJobRead) {
		return
	}

	requestCtx := request.Context()
	jobs, err := controlPlaneHandler.jobManager.ListJobs(requestCtx)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	listJobsResponse := ListJobsResponse{
		Jobs:  jobs,
		Count: len(jobs),
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, listJobsResponse)
}

// handleCreateJob handles POST /v1/_/tasks/jobs creating a new recurring cron job.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateJob(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleCreateJob invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksJobWrite) {
		return
	}

	var createJobInput CreateJobInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createJobInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	requestCtx := request.Context()
	job, err := controlPlaneHandler.jobManager.CreateJob(requestCtx, createJobInput)
	if err != nil {
		if errors.Is(err, ErrJobAlreadyExists) {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, err.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusCreated, job)
}

// handleGetJob handles GET /v1/_/tasks/jobs/{id} returning a single job by UUID.
func (controlPlaneHandler *ControlPlaneHandler) handleGetJob(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetJob invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksJobRead) {
		return
	}

	jobID, parseErr := uuid.Parse(request.PathValue("id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid job UUID")
		return
	}

	requestCtx := request.Context()
	jobWithCountdown, err := controlPlaneHandler.jobManager.GetJob(requestCtx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, jobWithCountdown)
}

// handleUpdateJob handles PATCH /v1/_/tasks/jobs/{id} updating an existing cron job.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateJob(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateJob invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksJobWrite) {
		return
	}

	jobID, parseErr := uuid.Parse(request.PathValue("id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid job UUID")
		return
	}

	var updateJobInput UpdateJobInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateJobInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	requestCtx := request.Context()
	job, err := controlPlaneHandler.jobManager.UpdateJob(requestCtx, jobID, updateJobInput)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrJobAlreadyExists) {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, err.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, job)
}

// handleDeleteJob handles DELETE /v1/_/tasks/jobs/{id} removing a job and cancelling pending executions.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteJob(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleDeleteJob invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksJobWrite) {
		return
	}

	jobID, parseErr := uuid.Parse(request.PathValue("id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid job UUID")
		return
	}

	requestCtx := request.Context()
	err := controlPlaneHandler.jobManager.DeleteJob(requestCtx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
