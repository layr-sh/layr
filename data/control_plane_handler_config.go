package data

import (
	"encoding/json"
	"net/http"
	"time"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:config.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	if controlPlaneHandler.configManager != nil {
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DefaultConfig())
}

// handleUpdateConfig handles PUT /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:config.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	if controlPlaneHandler.configManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
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

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(request.Context(), NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(controlPlaneHandler.configManager.Get())))
	}
}

// handleFlushCache handles POST /v1/_/data/cache/flush.
func (controlPlaneHandler *ControlPlaneHandler) handleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	if !controlPlaneHandler.checkScope(request, "data:cache.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}

	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateCache(request.Context(), InvalidateCacheInput{All: true, Catalog: true})
	}

	now := time.Now().UTC()
	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(request.Context(), NewCacheFlushedEvent("*", CacheFlushedEventData{
			Pattern:   "*",
			FlushedAt: now,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, FlushCacheResponse{
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
	if !controlPlaneHandler.checkScope(request, "data:cache.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
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

	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateCache(request.Context(), invalidateCacheInput)
	}

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(request.Context(), NewCacheInvalidatedEvent(pattern, CacheInvalidatedEventData{
			Pattern: pattern,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, InvalidateCacheResponse{
		Status:  "ok",
		Pattern: pattern,
	})
}
