// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/tasks/stats returning operational telemetry statistics.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get tasks statistics request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeTasksStatsRead) {
		return
	}

	requestCtx := request.Context()
	statsResponse, err := controlPlaneHandler.jobManager.GetStats(requestCtx)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, statsResponse)
}
