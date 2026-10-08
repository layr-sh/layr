// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"encoding/json"
	"errors"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/console/config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get console configuration request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleConfigRead) {
		return
	}

	log.Debug("retrieved console configuration")
	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
}

// handleUpdateConfig handles PUT /v1/_/console/config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update console configuration request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleConfigWrite) {
		return
	}

	config := controlPlaneHandler.configManager.Get()
	if decodeErr := json.NewDecoder(request.Body).Decode(&config); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if setErr := controlPlaneHandler.configManager.Set(request.Context(), config); setErr != nil {
		if errors.Is(setErr, ErrInvalidConfig) {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, setErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, setErr.Error())
		return
	}

	log.Debug("updated console configuration")
	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
}
