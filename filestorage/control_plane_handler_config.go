package filestorage

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/file-storage/config returning runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, core.ScopeFileStorageConfigRead) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}

	if controlPlaneHandler.configManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
		return
	}

	config := controlPlaneHandler.configManager.Get()
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, config)
}

// handleUpdateConfig handles PUT /v1/_/file-storage/config updating runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, core.ScopeFileStorageConfigWrite) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}

	if controlPlaneHandler.configManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
		return
	}

	var newConfig Config
	if decodeErr := json.NewDecoder(request.Body).Decode(&newConfig); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := request.Context()
	setErr := controlPlaneHandler.configManager.Set(ctx, newConfig)
	if setErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, setErr.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewConfigUpdatedEvent("file_storage.config", ConfigUpdatedEventData(newConfig)))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, newConfig)
}
