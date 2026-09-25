// Package function defines the serverless and edge function execution engine.
package function

import (
	"encoding/json"
	"errors"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/function/config returning dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get function configuration request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionConfigRead) {
		return
	}

	config := controlPlaneHandler.configManager.Get()
	core.WriteJSONResponse(responseWriter, http.StatusOK, config)
}

// handleUpdateConfig handles PUT /v1/_/function/config updating dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update function configuration request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionConfigWrite) {
		return
	}

	newConfig := controlPlaneHandler.configManager.Get()
	if decodeErr := json.NewDecoder(request.Body).Decode(&newConfig); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := request.Context()
	setErr := controlPlaneHandler.configManager.Set(ctx, newConfig)
	if setErr != nil {
		if errors.Is(setErr, ErrInvalidConfig) {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, setErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, setErr.Error())
		return
	}

	log.Debug("successfully updated function configuration")
	core.WriteJSONResponse(responseWriter, http.StatusOK, newConfig)
}
