package image

import (
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/image/stats returning cache metrics and memory stats.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleGetStats invoked")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImageStatsRead) {
		return
	}

	statsResponse := controlPlaneHandler.cacheManager.Stats()
	log.Debugf("retrieved cache statistics (hits: %d, misses: %d, ratio: %.2f)", statsResponse.CacheHits, statsResponse.CacheMisses, statsResponse.CacheHitRatio)
	core.WriteJSONResponse(responseWriter, http.StatusOK, statsResponse)
}
