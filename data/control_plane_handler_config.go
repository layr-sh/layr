package data

import (
	"encoding/json"
	"net/http"
	"time"
)

// HandleGetConfig handles GET /api/v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) HandleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:config.read") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	if controlPlaneHandler.configManager != nil {
		controlPlaneHandler.configManager.HandleGetConfig(responseWriter, request)
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DefaultConfig())
}

// HandleUpdateConfig handles PUT /api/v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) HandleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:config.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	if controlPlaneHandler.configManager == nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
		return
	}

	controlPlaneHandler.configManager.HandlePutConfig(responseWriter, request)

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(request.Context(), NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(controlPlaneHandler.configManager.Get())))
	}
}

// HandleFlushCache handles POST /api/v1/_/data/cache/flush.
func (controlPlaneHandler *ControlPlaneHandler) HandleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	if !controlPlaneHandler.checkScope(request, "data:cache.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}

	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateCache(request.Context(), InvalidateCacheRequest{All: true, Catalog: true})
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

// HandleInvalidateCache handles POST /api/v1/_/data/cache/invalidate.
func (controlPlaneHandler *ControlPlaneHandler) HandleInvalidateCache(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	if !controlPlaneHandler.checkScope(request, "data:cache.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}

	var invalidateCacheRequest InvalidateCacheRequest
	if request.Body != nil {
		if decodeErr := json.NewDecoder(request.Body).Decode(&invalidateCacheRequest); decodeErr != nil && decodeErr.Error() != "EOF" {
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
			return
		}
	}

	pattern := "*"
	if invalidateCacheRequest.Schema != "" && invalidateCacheRequest.Table != "" {
		pattern = invalidateCacheRequest.Schema + "." + invalidateCacheRequest.Table
	} else if invalidateCacheRequest.Pattern != "" {
		pattern = invalidateCacheRequest.Pattern
	}

	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateCache(request.Context(), invalidateCacheRequest)
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
