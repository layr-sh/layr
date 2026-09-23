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
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("List all recurring cron jobs"),
		core.RouteDescription("Returns all registered recurring cron jobs with next run countdowns."),
		core.RouteOperationID("tasks__jobs__list"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Job, CreateJobInput](router, "/v1/_/tasks/jobs", service.controlPlaneHandler.handleCreateJob,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Register a new recurring cron job"),
		core.RouteDescription("Creates a new recurring cron job schedule definition with target parameters."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("tasks__jobs__create"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[JobWithCountdown](router, "/v1/_/tasks/jobs/{id}", service.controlPlaneHandler.handleGetJob,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Get cron job details by ID"),
		core.RouteDescription("Returns a single cron job definition with next execution countdown."),
		core.RouteOperationID("tasks__jobs__get"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("get"),
	)
	core.PatchRoute[Job, UpdateJobInput](router, "/v1/_/tasks/jobs/{id}", service.controlPlaneHandler.handleUpdateJob,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Update an existing cron job"),
		core.RouteDescription("Modifies schedule expressions, timezone, payloads, or active status."),
		core.RouteOperationID("tasks__jobs__update"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/tasks/jobs/{id}", service.controlPlaneHandler.handleDeleteJob,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Delete a cron job"),
		core.RouteDescription("Deletes a job definition and cancels associated pending queued executions."),
		core.RouteNoContentResponse("Job deleted"),
		core.RouteOperationID("tasks__jobs__delete"),
		core.RouteSDKGroupName("tasks", "jobs"),
		core.RouteSDKMethodName("delete"),
	)

	// 2. Execution Triggers and Querying
	core.PostRoute[TriggerExecutionResponse, TriggerExecutionInput](router, "/v1/_/tasks/executions", service.controlPlaneHandler.handleTriggerExecution,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Trigger an immediate execution"),
		core.RouteDescription("Manually enqueues an execution for immediate processing out-of-schedule."),
		core.RouteDefaultStatusCode(http.StatusAccepted),
		core.RouteOperationID("tasks__executions__create"),
		core.RouteSDKGroupName("tasks", "executions"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/tasks/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Query execution audit logs"),
		core.RouteDescription("Returns historical execution status logs, durations, and errors."),
		core.RouteOperationID("tasks__executions__list"),
		core.RouteSDKGroupName("tasks", "executions"),
		core.RouteSDKMethodName("list"),
	)

	// 3. Dead-Letter Queue (DLQ) Management
	core.GetRoute[ListDLQResponse](router, "/v1/_/tasks/dlq", service.controlPlaneHandler.handleListDLQ,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("List failed tasks in DLQ"),
		core.RouteDescription("Inspects dead-lettered executions that exhausted all retry attempts."),
		core.RouteOperationID("tasks__dlq__list"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[RetryDLQResponse, core.Empty](router, "/v1/_/tasks/dlq/{id}/retry", service.controlPlaneHandler.handleRetryDLQ,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Re-queue a failed task from DLQ"),
		core.RouteDescription("Resets a dead-lettered execution to pending for immediate re-attempt."),
		core.RouteOperationID("tasks__dlq__retry"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("retry"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/tasks/dlq/{id}", service.controlPlaneHandler.handlePurgeDLQ,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Purge a task from DLQ"),
		core.RouteDescription("Permanently removes a failed execution from the dead-letter queue."),
		core.RouteNoContentResponse("DLQ execution purged"),
		core.RouteOperationID("tasks__dlq__purge"),
		core.RouteSDKGroupName("tasks", "dlq"),
		core.RouteSDKMethodName("purge"),
	)

	// 4. Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/tasks/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Get runtime tasks configuration"),
		core.RouteDescription("Returns dynamic concurrency limits, timeout guards, and backoff settings."),
		core.RouteOperationID("tasks__config__get"),
		core.RouteSDKGroupName("tasks", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/tasks/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Update runtime tasks configuration"),
		core.RouteDescription("Updates dynamic operational parameters including concurrency and retry backoff limits."),
		core.RouteOperationID("tasks__config__update"),
		core.RouteSDKGroupName("tasks", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 5. Telemetry Statistics
	core.GetRoute[StatsResponse](router, "/v1/_/tasks/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Tasks Control Plane"),
		core.RouteSummary("Get operational telemetry statistics"),
		core.RouteDescription("Returns real-time job counts, queue depths, DLQ sizes, and success rates."),
		core.RouteOperationID("tasks__stats__get"),
		core.RouteSDKGroupName("tasks", "stats"),
		core.RouteSDKMethodName("get"),
	)
}
