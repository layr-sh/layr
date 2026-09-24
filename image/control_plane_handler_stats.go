package image

import (
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/image/stats returning cache metrics and memory stats.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get image engine statistics request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImageStatsRead) {
		return
	}

	getStatsResponse := controlPlaneHandler.cacheManager.Stats()
	log.Debugf("retrieved cache statistics (hits: %d, misses: %d, ratio: %.2f)", getStatsResponse.CacheHits, getStatsResponse.CacheMisses, getStatsResponse.CacheHitRatio)
	core.WriteJSONResponse(responseWriter, http.StatusOK, getStatsResponse)
}
