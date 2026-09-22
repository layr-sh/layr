package data

import (
	"encoding/json"
	"net/http"
	"time"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataConfigRead) {
		return
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
}

// handleUpdateConfig handles PUT /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataConfigWrite) {
		return
	}

	var config Config
	if decodeErr := json.NewDecoder(request.Body).Decode(&config); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if setErr := controlPlaneHandler.configManager.Set(request.Context(), config); setErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, setErr.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(controlPlaneHandler.configManager.Get())))
}

// handleFlushCache handles POST /v1/_/data/cache/flush.
func (controlPlaneHandler *ControlPlaneHandler) handleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataCacheWrite) {
		return
	}

	controlPlaneHandler.InvalidateCache(request.Context(), InvalidateCacheInput{All: true, Catalog: true})

	now := time.Now().UTC()
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewCacheFlushedEvent("*", CacheFlushedEventData{
		Pattern:   "*",
		FlushedAt: now,
	}))

	core.WriteJSONResponse(responseWriter, http.StatusOK, FlushCacheResponse{
		Status:    "ok",
		FlushedAt: now,
	})
}

// handleInvalidateCache handles POST /v1/_/data/cache/invalidate.
func (controlPlaneHandler *ControlPlaneHandler) handleInvalidateCache(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataCacheWrite) {
		return
	}

	var invalidateCacheInput InvalidateCacheInput
	if request.Body != nil {
		if decodeErr := json.NewDecoder(request.Body).Decode(&invalidateCacheInput); decodeErr != nil && decodeErr.Error() != "EOF" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
			return
		}
	}

	pattern := "*"
	if invalidateCacheInput.Schema != "" && invalidateCacheInput.Table != "" {
		pattern = invalidateCacheInput.Schema + "." + invalidateCacheInput.Table
	} else if invalidateCacheInput.Pattern != "" {
		pattern = invalidateCacheInput.Pattern
	}

	controlPlaneHandler.InvalidateCache(request.Context(), invalidateCacheInput)

	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewCacheInvalidatedEvent(pattern, CacheInvalidatedEventData{
		Pattern: pattern,
	}))

	core.WriteJSONResponse(responseWriter, http.StatusOK, InvalidateCacheResponse{
		Status:  "ok",
		Pattern: pattern,
	})
}
