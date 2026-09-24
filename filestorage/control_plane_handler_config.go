package filestorage

import (
	"encoding/json"
	"errors"
	"net/http"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/file-storage/config returning runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageConfigRead) {
		return
	}

	config := controlPlaneHandler.configManager.Get()
	core.WriteJSONResponse(responseWriter, http.StatusOK, config)
}

// handleUpdateConfig handles PUT /v1/_/file-storage/config updating runtime config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateConfig invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageConfigWrite) {
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

	log.Debug("handleUpdateConfig successfully saved dynamic file storage configuration")

	core.WriteJSONResponse(responseWriter, http.StatusOK, newConfig)
}
