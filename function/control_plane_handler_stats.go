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

	getStatsResponse := GetStatsResponse{Runtimes: healthMap}
	// Aggregate each relation independently so deployments and routes do not multiply execution totals.
	const queryStatsSQL = `
		WITH recent_executions AS (
			SELECT *, (status_code >= 400 OR NULLIF(error_message, '') IS NOT NULL) AS failed
			FROM function.executions
			WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours' AND created_at <= CURRENT_TIMESTAMP
		), endpoint_usage AS (
			SELECT e.name, e.runtime, e.is_public, (e.active_deployment_id IS NOT NULL) AS is_deployed,
			       COUNT(x.id) AS executions, COUNT(x.id) FILTER (WHERE x.failed) AS failed_executions,
			       AVG(x.duration_ms) AS average_duration_ms,
			       percentile_cont(0.95) WITHIN GROUP (ORDER BY x.duration_ms) AS p95_duration_ms,
			       MAX(x.created_at) AS last_executed_at
			FROM function.endpoints e LEFT JOIN recent_executions x ON x.endpoint_id = e.id
			GROUP BY e.id
		)
		SELECT
			(SELECT COUNT(*) FROM function.endpoints),
			(SELECT COUNT(*) FROM function.endpoints WHERE is_public),
			(SELECT COUNT(*) FROM function.endpoints WHERE NOT is_public),
			(SELECT COUNT(*) FROM function.endpoints WHERE active_deployment_id IS NOT NULL),
			(SELECT COUNT(*) FROM function.deployments),

			(SELECT COALESCE(SUM(octet_length(bundle_content)), 0) FROM function.deployments),
			(SELECT COUNT(*) FROM function.custom_domains),
			(SELECT COUNT(*) FROM function.custom_domain_routes),
			(SELECT COUNT(*) FROM function.custom_domains WHERE tls_enabled AND (tls_status <> 'active' OR tls_expires_at <= CURRENT_TIMESTAMP)),
			(SELECT COUNT(*) FROM function.executions),
			COUNT(*), COUNT(*) FILTER (WHERE failed),
			COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500),
			COUNT(*) FILTER (WHERE status_code >= 500),
			AVG(duration_ms), percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms),
			(SELECT MAX(created_at) FROM function.executions WHERE created_at <= CURRENT_TIMESTAMP),
			CURRENT_TIMESTAMP - INTERVAL '24 hours', CURRENT_TIMESTAMP,
			(SELECT COALESCE(jsonb_agg(to_jsonb(endpoint_usage) ORDER BY executions DESC, name), '[]'::jsonb) FROM endpoint_usage)
		FROM recent_executions;
	`
	if queryErr := controlPlaneHandler.kernel.DB().QueryRow(requestCtx, queryStatsSQL).Scan(
		&getStatsResponse.TotalEndpoints, &getStatsResponse.PublicEndpoints, &getStatsResponse.PrivateEndpoints,
		&getStatsResponse.DeployedEndpoints, &getStatsResponse.TotalDeployments,
		&getStatsResponse.BundleSizeBytes, &getStatsResponse.TotalDomains, &getStatsResponse.DomainRoutes,
		&getStatsResponse.DomainsNeedingTLS, &getStatsResponse.TotalExecutions, &getStatsResponse.ExecutionsLast24Hours,
		&getStatsResponse.FailedExecutionsLast24Hours, &getStatsResponse.ClientErrorsLast24Hours, &getStatsResponse.ServerErrorsLast24Hours,
		&getStatsResponse.AverageDurationMs, &getStatsResponse.P95DurationMs, &getStatsResponse.LastExecutedAt,
		&getStatsResponse.WindowStartedAt, &getStatsResponse.GeneratedAt, &getStatsResponse.Endpoints,
	); queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, getStatsResponse)
}
