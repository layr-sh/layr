// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/tasks/config returning dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksConfigRead) {
		return
	}

	tasksConfig := controlPlaneHandler.configManager.Get()
	core.WriteJSONResponse(responseWriter, http.StatusOK, tasksConfig)
}

// handleUpdateConfig handles PUT /v1/_/tasks/config updating dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksConfigWrite) {
		return
	}

	var newConfig Config
	if err := json.NewDecoder(request.Body).Decode(&newConfig); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	requestCtx := request.Context()
	if err := controlPlaneHandler.configManager.Set(requestCtx, newConfig); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, newConfig)
}
