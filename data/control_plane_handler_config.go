package data

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"layr.sh/core"
)

// handleGetConfig handles GET /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetConfig invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataConfigRead) {
		return
	}
	log.Debug("retrieved data configuration")
	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
}

// handleUpdateConfig handles PUT /v1/_/data/config.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateConfig invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataConfigWrite) {
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

	log.Debug("successfully updated data configuration")
	core.WriteJSONResponse(responseWriter, http.StatusOK, controlPlaneHandler.configManager.Get())
}

// handleFlushCache handles POST /v1/_/data/cache/flush.
func (controlPlaneHandler *ControlPlaneHandler) handleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleFlushCache invoked")

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

	log.Debug("successfully flushed data cache")
	core.WriteJSONResponse(responseWriter, http.StatusOK, FlushCacheResponse{
		Status:    "ok",
		FlushedAt: now,
	})
}

// handleInvalidateCache handles POST /v1/_/data/cache/invalidate.
func (controlPlaneHandler *ControlPlaneHandler) handleInvalidateCache(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleInvalidateCache invoked")

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

	log.Debugf("successfully invalidated data cache for pattern %s", pattern)
	core.WriteJSONResponse(responseWriter, http.StatusOK, InvalidateCacheResponse{
		Status:  "ok",
		Pattern: pattern,
	})
}
