// Package function defines the serverless and edge function execution engine.
package function

import (
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/function/stats returning telemetry and runner health.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get function statistics request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionStatsRead) {
		return
	}

	requestCtx := request.Context()
	healthMap, _ := controlPlaneHandler.engine.Health(requestCtx)

	var totalEndpoints int
	const queryCountSQL = `SELECT COUNT(*) FROM function.endpoints;`
	if queryErr := controlPlaneHandler.kernel.DB().QueryRow(requestCtx, queryCountSQL).Scan(&totalEndpoints); queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, GetStatsResponse{
		Runtimes:       healthMap,
		TotalEndpoints: totalEndpoints,
	})
}
