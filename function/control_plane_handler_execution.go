// Package function defines the serverless and edge function execution engine.
package function

import (
	"net/http"
	"strconv"

	"uuid"

	"layr.sh/core"
)

const defaultExecutionListLimit = 50

// handleListExecutions handles GET /v1/_/function/executions and GET /v1/_/function/endpoints/{endpoint_id}/executions.
func (controlPlaneHandler *ControlPlaneHandler) handleListExecutions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list executions request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	var endpointID *uuid.UUID

	// Check path parameter {endpoint_id}
	if endpointIDString := request.PathValue("endpoint_id"); endpointIDString != "" {
		parsedID, parseErr := uuid.Parse(endpointIDString)
		if parseErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
			return
		}
		endpointID = &parsedID
	} else if rawEndpointID := request.URL.Query().Get("endpoint_id"); rawEndpointID != "" {
		parsedID, parseErr := uuid.Parse(rawEndpointID)
		if parseErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint_id query parameter")
			return
		}
		endpointID = &parsedID
	}

	limit := defaultExecutionListLimit
	if rawLimit := request.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, convErr := strconv.Atoi(rawLimit); convErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if rawOffset := request.URL.Query().Get("offset"); rawOffset != "" {
		if parsedOffset, convErr := strconv.Atoi(rawOffset); convErr == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	var countQuery string
	var selectQuery string
	var countArguments []any
	var selectArguments []any

	if endpointID != nil {
		countQuery = `SELECT COUNT(*) FROM function.executions WHERE endpoint_id = $1;`
		countArguments = []any{*endpointID}
		selectQuery = `
			SELECT id, endpoint_id, deployment_id, method, path, status_code, duration_ms, stdout, stderr, error_message, created_at
			FROM function.executions
			WHERE endpoint_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3;
		`
		selectArguments = []any{*endpointID, limit, offset}
	} else {
		countQuery = `SELECT COUNT(*) FROM function.executions;`
		countArguments = nil
		selectQuery = `
			SELECT id, endpoint_id, deployment_id, method, path, status_code, duration_ms, stdout, stderr, error_message, created_at
			FROM function.executions
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2;
		`
		selectArguments = []any{limit, offset}
	}

	var totalCount int
	_ = controlPlaneHandler.kernel.DB().QueryRow(request.Context(), countQuery, countArguments...).Scan(&totalCount)

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), selectQuery, selectArguments...)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	executions := make([]Execution, 0)
	for rows.Next() {
		var execution Execution
		_ = rows.Scan(
			&execution.ID,
			&execution.EndpointID,
			&execution.DeploymentID,
			&execution.Method,
			&execution.Path,
			&execution.StatusCode,
			&execution.DurationMs,
			&execution.Stdout,
			&execution.Stderr,
			&execution.ErrorMessage,
			&execution.CreatedAt,
		)
		executions = append(executions, execution)
	}

	log.Debugf("retrieved %d execution(s)", len(executions))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListExecutionsResponse{
		Executions: executions,
		Count:      totalCount,
	})
}
