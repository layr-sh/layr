// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"uuid"

	"layr.sh/core"
)

const (
	defaultListLimit = 50
)

// handleTriggerExecution handles POST /v1/_/tasks/executions triggering an immediate execution.
func (controlPlaneHandler *ControlPlaneHandler) handleTriggerExecution(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling trigger execution request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksExecutionWrite) {
		return
	}

	var triggerExecutionInput TriggerExecutionInput
	if request.Body != nil && request.ContentLength > 0 {
		if decodeErr := json.NewDecoder(request.Body).Decode(&triggerExecutionInput); decodeErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	requestCtx := request.Context()
	triggerExecutionResponse, err := controlPlaneHandler.jobManager.TriggerExecution(requestCtx, triggerExecutionInput)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Job not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	log.Debugf("execution %s successfully triggered for job %s", triggerExecutionResponse.ExecutionID, triggerExecutionInput.JobID)
	core.WriteJSONResponse(responseWriter, http.StatusAccepted, triggerExecutionResponse)
}

// handleListExecutions handles GET /v1/_/tasks/executions querying execution history logs.
func (controlPlaneHandler *ControlPlaneHandler) handleListExecutions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list executions request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksExecutionRead) {
		return
	}

	queryValues := request.URL.Query()
	var jobID *uuid.UUID
	if rawJobID := queryValues.Get("job_id"); rawJobID != "" {
		parsedID, parseErr := uuid.Parse(rawJobID)
		if parseErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid job_id query parameter")
			return
		}
		jobID = &parsedID
	}

	limit := defaultListLimit
	if rawLimit := queryValues.Get("limit"); rawLimit != "" {
		if parsedLimit, convErr := strconv.Atoi(rawLimit); convErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if rawOffset := queryValues.Get("offset"); rawOffset != "" {
		if parsedOffset, convErr := strconv.Atoi(rawOffset); convErr == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	requestCtx := request.Context()
	executions, totalCount, err := controlPlaneHandler.jobManager.ListExecutions(requestCtx, jobID, limit, offset)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	listExecutionsResponse := ListExecutionsResponse{
		Executions: executions,
		Count:      totalCount,
	}
	log.Debugf("retrieved %d execution(s)", len(executions))
	core.WriteJSONResponse(responseWriter, http.StatusOK, listExecutionsResponse)
}

// handleListDLQ handles GET /v1/_/tasks/dlq returning dead-lettered executions.
func (controlPlaneHandler *ControlPlaneHandler) handleListDLQ(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list DLQ executions request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksExecutionRead) {
		return
	}

	queryValues := request.URL.Query()
	limit := defaultListLimit
	if rawLimit := queryValues.Get("limit"); rawLimit != "" {
		if parsedLimit, convErr := strconv.Atoi(rawLimit); convErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if rawOffset := queryValues.Get("offset"); rawOffset != "" {
		if parsedOffset, convErr := strconv.Atoi(rawOffset); convErr == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	requestCtx := request.Context()
	dlq, totalCount, err := controlPlaneHandler.jobManager.ListDLQ(requestCtx, limit, offset)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	listDLQResponse := ListDLQResponse{
		DLQ:   dlq,
		Count: totalCount,
	}
	log.Debugf("retrieved %d DLQ execution(s)", len(dlq))
	core.WriteJSONResponse(responseWriter, http.StatusOK, listDLQResponse)
}

// handleRetryDLQ handles POST /v1/_/tasks/dlq/{execution_id}/retry re-queuing a failed execution.
func (controlPlaneHandler *ControlPlaneHandler) handleRetryDLQ(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling retry DLQ execution request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksExecutionWrite) {
		return
	}

	executionID, parseErr := uuid.Parse(request.PathValue("execution_id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid execution UUID")
		return
	}

	requestCtx := request.Context()
	retryDLQResponse, err := controlPlaneHandler.jobManager.RetryDLQExecution(requestCtx, executionID)
	if err != nil {
		if errors.Is(err, ErrDLQItemNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Execution not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	log.Debugf("retried DLQ execution %s", executionID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, retryDLQResponse)
}

// handlePurgeDLQ handles DELETE /v1/_/tasks/dlq/{execution_id} deleting a failed execution from DLQ.
func (controlPlaneHandler *ControlPlaneHandler) handlePurgeDLQ(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling purge DLQ executions request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksExecutionWrite) {
		return
	}

	executionID, parseErr := uuid.Parse(request.PathValue("execution_id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid execution UUID")
		return
	}

	requestCtx := request.Context()
	err := controlPlaneHandler.jobManager.PurgeDLQExecution(requestCtx, executionID)
	if err != nil {
		if errors.Is(err, ErrDLQItemNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Execution not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	log.Debugf("purged DLQ execution %s", executionID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
