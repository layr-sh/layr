package image

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/image/config returning dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImageConfigRead) {
		return
	}

	config := controlPlaneHandler.configManager.Get()
	core.WriteJSONResponse(responseWriter, http.StatusOK, config)
}

// handleUpdateConfig handles PUT /v1/_/image/config updating dynamic runtime configuration.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImageConfigWrite) {
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

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("image.config", ConfigUpdatedEventData(newConfig)))
	log.Debug("handleUpdateConfig successfully saved dynamic image configuration")

	core.WriteJSONResponse(responseWriter, http.StatusOK, newConfig)
}
