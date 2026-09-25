// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

var validEndpointNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// fetchEndpointByID retrieves an endpoint definition by UUID.
func (controlPlaneHandler *ControlPlaneHandler) fetchEndpointByID(ctx context.Context, endpointID uuid.UUID) (*Endpoint, error) {
	const querySQL = `
		SELECT id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at
		FROM function.endpoints
		WHERE id = $1;
	`

	var storedEndpoint Endpoint
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, querySQL, endpointID).Scan(
		&storedEndpoint.ID,
		&storedEndpoint.Name,
		&storedEndpoint.Description,
		&storedEndpoint.Runtime,
		&storedEndpoint.Entrypoint,
		&storedEndpoint.MemoryLimitMB,
		&storedEndpoint.TimeoutSeconds,
		&storedEndpoint.IsPublic,
		&storedEndpoint.ActiveDeploymentID,
		&storedEndpoint.CreatedAt,
		&storedEndpoint.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) || errors.Is(scanErr, sql.ErrNoRows) {
			return nil, ErrEndpointNotFound
		}
		return nil, fmt.Errorf("failed to query endpoint: %w", scanErr)
	}

	return &storedEndpoint, nil
}

// handleListEndpoints handles GET /v1/_/function/endpoints returning all registered endpoints.
func (controlPlaneHandler *ControlPlaneHandler) handleListEndpoints(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list endpoints request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	const querySQL = `
		SELECT id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at
		FROM function.endpoints
		ORDER BY name ASC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	endpoints := make([]Endpoint, 0)
	for rows.Next() {
		var storedEndpoint Endpoint
		_ = rows.Scan(
			&storedEndpoint.ID,
			&storedEndpoint.Name,
			&storedEndpoint.Description,
			&storedEndpoint.Runtime,
			&storedEndpoint.Entrypoint,
			&storedEndpoint.MemoryLimitMB,
			&storedEndpoint.TimeoutSeconds,
			&storedEndpoint.IsPublic,
			&storedEndpoint.ActiveDeploymentID,
			&storedEndpoint.CreatedAt,
			&storedEndpoint.UpdatedAt,
		)
		endpoints = append(endpoints, storedEndpoint)
	}

	log.Debugf("retrieved %d endpoint(s)", len(endpoints))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListEndpointsResponse{
		Endpoints: endpoints,
		Count:     len(endpoints),
	})
}

// handleCreateEndpoint handles POST /v1/_/function/endpoints registering a new endpoint definition.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create endpoint request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	var createEndpointInput CreateEndpointInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createEndpointInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	name := strings.TrimSpace(createEndpointInput.Name)
	if !validEndpointNamePattern.MatchString(name) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, ErrInvalidEndpointName.Error())
		return
	}

	runtimeName := strings.TrimSpace(createEndpointInput.Runtime)
	if runtimeName == "" {
		runtimeName = controlPlaneHandler.configManager.Get().DefaultRuntime
	}

	entrypoint := strings.TrimSpace(createEndpointInput.Entrypoint)
	if entrypoint == "" {
		entrypoint = "index.js"
	}

	memoryLimitMB := createEndpointInput.MemoryLimitMB
	if memoryLimitMB <= 0 {
		memoryLimitMB = controlPlaneHandler.configManager.Get().DefaultMemoryLimitMB
	}

	timeoutSeconds := createEndpointInput.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = controlPlaneHandler.configManager.Get().DefaultTimeoutSeconds
	}

	isPublic := true
	if createEndpointInput.IsPublic != nil {
		isPublic = *createEndpointInput.IsPublic
	}

	const insertSQL = `
		INSERT INTO function.endpoints (
			name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, clock_timestamp(), clock_timestamp()
		)
		RETURNING id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at;
	`

	var storedEndpoint Endpoint
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(
		request.Context(),
		insertSQL,
		name,
		createEndpointInput.Description,
		runtimeName,
		entrypoint,
		memoryLimitMB,
		timeoutSeconds,
		isPublic,
	).Scan(
		&storedEndpoint.ID,
		&storedEndpoint.Name,
		&storedEndpoint.Description,
		&storedEndpoint.Runtime,
		&storedEndpoint.Entrypoint,
		&storedEndpoint.MemoryLimitMB,
		&storedEndpoint.TimeoutSeconds,
		&storedEndpoint.IsPublic,
		&storedEndpoint.ActiveDeploymentID,
		&storedEndpoint.CreatedAt,
		&storedEndpoint.UpdatedAt,
	)
	if scanErr != nil {
		if strings.Contains(scanErr.Error(), "duplicate key") || strings.Contains(scanErr.Error(), "unique constraint") {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, ErrEndpointAlreadyExists.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewEndpointCreatedEvent(storedEndpoint.ID.String(), EndpointCreatedEventData(storedEndpoint)),
	)

	log.Debugf("endpoint %s successfully created", storedEndpoint.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, storedEndpoint)
}

// handleGetEndpoint handles GET /v1/_/function/endpoints/{id} returning an endpoint definition.
func (controlPlaneHandler *ControlPlaneHandler) handleGetEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get endpoint request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	storedEndpoint, getErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID)
	if getErr != nil {
		if errors.Is(getErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, getErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, getErr.Error())
		return
	}

	log.Debugf("retrieved endpoint %s", storedEndpoint.ID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, storedEndpoint)
}

// handleUpdateEndpoint handles PUT /v1/_/function/endpoints/{id} modifying endpoint metadata.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update endpoint request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	var updateEndpointInput UpdateEndpointInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateEndpointInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	existingEndpoint, fetchErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	if updateEndpointInput.Description != nil {
		existingEndpoint.Description = updateEndpointInput.Description
	}
	if updateEndpointInput.Runtime != nil && strings.TrimSpace(*updateEndpointInput.Runtime) != "" {
		existingEndpoint.Runtime = strings.TrimSpace(*updateEndpointInput.Runtime)
	}
	if updateEndpointInput.Entrypoint != nil && strings.TrimSpace(*updateEndpointInput.Entrypoint) != "" {
		existingEndpoint.Entrypoint = strings.TrimSpace(*updateEndpointInput.Entrypoint)
	}
	if updateEndpointInput.MemoryLimitMB != nil && *updateEndpointInput.MemoryLimitMB > 0 {
		existingEndpoint.MemoryLimitMB = *updateEndpointInput.MemoryLimitMB
	}
	if updateEndpointInput.TimeoutSeconds != nil && *updateEndpointInput.TimeoutSeconds > 0 {
		existingEndpoint.TimeoutSeconds = *updateEndpointInput.TimeoutSeconds
	}
	if updateEndpointInput.IsPublic != nil {
		existingEndpoint.IsPublic = *updateEndpointInput.IsPublic
	}

	const updateSQL = `
		UPDATE function.endpoints
		SET description = $1, runtime = $2, entrypoint = $3, memory_limit_mb = $4, timeout_seconds = $5, is_public = $6, updated_at = clock_timestamp()
		WHERE id = $7
		RETURNING updated_at;
	`

	queryErr := controlPlaneHandler.kernel.DB().QueryRow(
		request.Context(),
		updateSQL,
		existingEndpoint.Description,
		existingEndpoint.Runtime,
		existingEndpoint.Entrypoint,
		existingEndpoint.MemoryLimitMB,
		existingEndpoint.TimeoutSeconds,
		existingEndpoint.IsPublic,
		endpointID,
	).Scan(&existingEndpoint.UpdatedAt)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewEndpointUpdatedEvent(existingEndpoint.ID.String(), EndpointUpdatedEventData(*existingEndpoint)),
	)

	log.Debugf("endpoint %s successfully updated", existingEndpoint.ID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, existingEndpoint)
}

// handleDeleteEndpoint handles DELETE /v1/_/function/endpoints/{id} removing an endpoint definition.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete endpoint request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	existingEndpoint, fetchErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	// Undeploy from runtime runner
	_ = controlPlaneHandler.engine.Undeploy(request.Context(), existingEndpoint.Runtime, existingEndpoint.Name)

	const deleteSQL = `DELETE FROM function.endpoints WHERE id = $1;`
	if _, queryErr := controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSQL, endpointID); queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewEndpointDeletedEvent(existingEndpoint.ID.String(), EndpointDeletedEventData(*existingEndpoint)),
	)

	log.Debugf("endpoint %s successfully deleted", existingEndpoint.ID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
