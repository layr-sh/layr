package filestorage

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/file-storage/config returning runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageConfigRead) {
		return
	}

	config := controlPlaneHandler.configManager.Get()
	core.WriteJSONResponse(responseWriter, http.StatusOK, config)
}

// handleUpdateConfig handles PUT /v1/_/file-storage/config updating runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageConfigWrite) {
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

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("file_storage.config", ConfigUpdatedEventData(newConfig)))

	core.WriteJSONResponse(responseWriter, http.StatusOK, newConfig)
}
