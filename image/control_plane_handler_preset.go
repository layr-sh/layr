package image

import (
	"encoding/json"
	"errors"
	"net/http"

	"uuid"

	"layr.sh/core"
)

// handleListPresets handles GET /v1/_/image/presets returning all presets.
func (controlPlaneHandler *ControlPlaneHandler) handleListPresets(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list image presets request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImagePresetRead) {
		return
	}

	ctx := request.Context()
	presets, listErr := controlPlaneHandler.presetManager.List(ctx)
	if listErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, listErr.Error())
		return
	}

	listPresetsResponse := ListPresetsResponse{
		Presets: presets,
		Count:   len(presets),
	}
	log.Debugf("retrieved %d preset(s)", len(presets))
	core.WriteJSONResponse(responseWriter, http.StatusOK, listPresetsResponse)
}

// handleCreatePreset handles POST /v1/_/image/presets creating a new transformation preset.
func (controlPlaneHandler *ControlPlaneHandler) handleCreatePreset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create image preset request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImagePresetWrite) {
		return
	}

	var createPresetInput CreatePresetInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createPresetInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := request.Context()
	preset, createErr := controlPlaneHandler.presetManager.Create(ctx, createPresetInput)
	if createErr != nil {
		if errors.Is(createErr, ErrPresetExists) {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, createErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewPresetCreatedEvent(preset.ID.String(), PresetCreatedEventData(*preset)))
	log.Debugf("preset %s successfully created", preset.ID)

	core.WriteJSONResponse(responseWriter, http.StatusCreated, preset)
}

// handleGetPreset handles GET /v1/_/image/presets/{preset_id} returning a preset by ID.
func (controlPlaneHandler *ControlPlaneHandler) handleGetPreset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get image preset request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImagePresetRead) {
		return
	}

	presetID, parseErr := uuid.Parse(request.PathValue("preset_id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid preset UUID")
		return
	}

	ctx := request.Context()
	preset, getErr := controlPlaneHandler.presetManager.GetByID(ctx, presetID)
	if getErr != nil {
		if errors.Is(getErr, ErrPresetNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Preset not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, getErr.Error())
		return
	}

	log.Debugf("retrieved preset %s", presetID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, preset)
}

// handleUpdatePreset handles PUT /v1/_/image/presets/{preset_id} modifying an existing preset.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdatePreset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update image preset request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImagePresetWrite) {
		return
	}

	presetID, parseErr := uuid.Parse(request.PathValue("preset_id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid preset UUID")
		return
	}

	var updatePresetInput UpdatePresetInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updatePresetInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := request.Context()
	preset, updateErr := controlPlaneHandler.presetManager.Update(ctx, presetID, updatePresetInput)
	if updateErr != nil {
		if errors.Is(updateErr, ErrPresetNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Preset not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, updateErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewPresetUpdatedEvent(preset.ID.String(), PresetUpdatedEventData(*preset)))
	log.Debugf("preset %s successfully updated", presetID)

	core.WriteJSONResponse(responseWriter, http.StatusOK, preset)
}

// handleDeletePreset handles DELETE /v1/_/image/presets/{preset_id} removing a preset.
func (controlPlaneHandler *ControlPlaneHandler) handleDeletePreset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete image preset request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImagePresetWrite) {
		return
	}

	presetID, parseErr := uuid.Parse(request.PathValue("preset_id"))
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid preset UUID")
		return
	}

	ctx := request.Context()
	presetName, deleteErr := controlPlaneHandler.presetManager.Delete(ctx, presetID)
	if deleteErr != nil {
		if errors.Is(deleteErr, ErrPresetNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Preset not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, deleteErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewPresetDeletedEvent(presetID.String(), PresetDeletedEventData{PresetName: presetName}))
	log.Debugf("preset %s successfully deleted", presetID)

	responseWriter.WriteHeader(http.StatusNoContent)
}
