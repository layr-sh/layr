// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Tasks control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Cron Job Definitions
	core.GetRoute[ListJobsResponse](router, "/v1/_/tasks/jobs", service.controlPlaneHandler.handleListJobs,
		core.RouteTag("Jobs"),
		core.RouteSummary("List all recurring cron jobs"),
		core.RouteDescription("Returns all registered recurring cron jobs from tasks.jobs, detailing cron schedule expressions, target drivers (http, function, sql), target payloads, timezone, next execution countdowns, and active status. Requires tasks:job.read scope."),
		core.RouteOperationID("tasks__jobs__list"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Job, CreateJobInput](router, "/v1/_/tasks/jobs", service.controlPlaneHandler.handleCreateJob,
		core.RouteTag("Jobs"),
		core.RouteSummary("Register a new recurring cron job"),
		core.RouteDescription("Creates a recurring cron job in tasks.jobs with 5-field cron expression, target execution driver (http webhook, serverless function, or SQL stored procedure), payload JSON, timeout guards, retry limits, and timezone. Calculates next run timestamp and emits tasks.job.created event. Requires tasks:job.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("tasks__jobs__create"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[GetJobResponse](router, "/v1/_/tasks/jobs/{job_id}", service.controlPlaneHandler.handleGetJob,
		core.RouteTag("Jobs"),
		core.RouteSummary("Get cron job definition by ID"),
		core.RouteDescription("Retrieves a single cron job definition from tasks.jobs by ID, including cron expression, target payload, execution driver, next run countdown, and status. Requires tasks:job.read scope."),
		core.RouteOperationID("tasks__jobs__get"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("get"),
	)
	core.PatchRoute[Job, UpdateJobInput](router, "/v1/_/tasks/jobs/{job_id}", service.controlPlaneHandler.handleUpdateJob,
		core.RouteTag("Jobs"),
		core.RouteSummary("Update an existing cron job definition"),
		core.RouteDescription("Modifies cron schedule expressions, timezone, target payloads, retry parameters, or active status in tasks.jobs. Recalculates next run timestamp and emits tasks.job.updated event. Requires tasks:job.write scope."),
		core.RouteOperationID("tasks__jobs__update"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/tasks/jobs/{job_id}", service.controlPlaneHandler.handleDeleteJob,
		core.RouteTag("Jobs"),
		core.RouteSummary("Delete a cron job and cancel queued executions"),
		core.RouteDescription("Deletes a cron job definition from tasks.jobs and cancels any associated pending executions in tasks.executions. Emits tasks.job.deleted event. Requires tasks:job.write scope."),
		core.RouteNoContentResponse("Job deleted"),
		core.RouteOperationID("tasks__jobs__delete"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("delete"),
	)

	// 2. Execution Triggers and Querying
	core.PostRoute[TriggerExecutionResponse, TriggerExecutionInput](router, "/v1/_/tasks/executions", service.controlPlaneHandler.handleTriggerExecution,
		core.RouteTag("Executions"),
		core.RouteSummary("Trigger an immediate out-of-schedule execution"),
		core.RouteDescription("Manually enqueues an execution for a job in tasks.executions with status 'pending' for immediate worker dispatch out-of-schedule. Requires tasks:execution.write scope."),
		core.RouteDefaultStatusCode(http.StatusAccepted),
		core.RouteOperationID("tasks__executions__create"),
		core.RouteSDKGroupName("tasks", "executions"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/tasks/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Executions"),
		core.RouteSummary("Query execution audit logs"),
		core.RouteDescription("Queries execution history records from tasks.executions with filtering by job ID, execution status (pending, running, completed, failed, dead_lettered), duration, and date ranges. Requires tasks:execution.read scope."),
		core.RouteDefaultPagination(),
		core.RouteQueryParam("job_id", "Filter task executions by job UUID"),
		core.RouteOperationID("tasks__executions__list"),
		core.RouteSDKGroupName("tasks", "executions"),
		core.RouteSDKMethodName("list"),
	)

	// 3. Dead-Letter Queue (DLQ) Management
	core.GetRoute[ListDLQResponse](router, "/v1/_/tasks/dlq", service.controlPlaneHandler.handleListDLQ,
		core.RouteTag("DLQ"),
		core.RouteSummary("List dead-lettered failed task executions"),
		core.RouteDescription("Queries dead-lettered executions from tasks.executions that exhausted all configured retry attempts, detailing failure errors, stack traces, and attempt counts. Requires tasks:execution.read scope."),
		core.RouteDefaultPagination(),
		core.RouteOperationID("tasks__dlq__list"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[RetryDLQResponse, core.Empty](router, "/v1/_/tasks/dlq/{execution_id}/retry", service.controlPlaneHandler.handleRetryDLQ,
		core.RouteTag("DLQ"),
		core.RouteSummary("Re-queue a failed task execution from the DLQ"),
		core.RouteDescription("Resets a dead-lettered execution in tasks.executions to 'pending' status for immediate worker retry pickup. Requires tasks:execution.write scope."),
		core.RouteOperationID("tasks__dlq__retry"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("retry"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/tasks/dlq/{execution_id}", service.controlPlaneHandler.handlePurgeDLQ,
		core.RouteTag("DLQ"),
		core.RouteSummary("Permanently purge a failed task from the DLQ"),
		core.RouteDescription("Permanently removes a failed execution record from tasks.executions / dead-letter queue. Requires tasks:execution.write scope."),
		core.RouteNoContentResponse("DLQ execution purged"),
		core.RouteOperationID("tasks__dlq__purge"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("purge"),
	)

	// 4. Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/tasks/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Get runtime tasks orchestrator configuration"),
		core.RouteDescription("Returns operational task configuration settings including worker concurrency limits, poller interval, default timeout guards, and exponential retry backoff parameters. Requires tasks:config.read scope."),
		core.RouteOperationID("tasks__config__get"),
		core.RouteSDKGroupName("tasks", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/tasks/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update runtime tasks orchestrator configuration"),
		core.RouteDescription("Updates operational task configuration in tasks.config, dynamically adjusting worker concurrency, polling intervals, and retry backoff limits without restarting workers. Requires tasks:config.write scope."),
		core.RouteOperationID("tasks__config__update"),
		core.RouteSDKGroupName("tasks", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 5. Telemetry Statistics
	core.GetRoute[GetStatsResponse](router, "/v1/_/tasks/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Stats"),
		core.RouteSummary("Get operational task telemetry and queue statistics"),
		core.RouteDescription("Returns real-time task statistics including total job counts, active queue depth, currently running executions, DLQ size, and execution success rate. Requires tasks:stats.read scope."),
		core.RouteOperationID("tasks__stats__get"),
		core.RouteSDKGroupName("tasks", "stats"),
		core.RouteSDKMethodName("get"),
	)
}
