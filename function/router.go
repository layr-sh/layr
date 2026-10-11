// Package function defines the serverless and edge function execution engine.
package function

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Function data plane and administrative control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	if service.kernel.Server() != nil {
		service.RegisterHostRoutes(service.kernel.Server())
	}
	service.registerBaseRoutes(baseRouter)
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// Proxy routes for subpaths
	core.GetRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy GET to endpoint"),
		core.RouteDescription("Proxies public HTTP GET requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__get"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy POST to endpoint"),
		core.RouteDescription("Proxies public HTTP POST requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__post"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("post"),
	)
	core.PutRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy PUT to endpoint"),
		core.RouteDescription("Proxies public HTTP PUT requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__put"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("put"),
	)
	core.DeleteRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy DELETE to endpoint"),
		core.RouteDescription("Proxies public HTTP DELETE requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__delete"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("delete"),
	)
	core.PatchRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy PATCH to endpoint"),
		core.RouteDescription("Proxies public HTTP PATCH requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__patch"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("patch"),
	)
	core.HeadRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy HEAD to endpoint"),
		core.RouteDescription("Proxies public HTTP HEAD requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__head"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("head"),
	)

	// Proxy routes for root endpoint path
	core.GetRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy GET to endpoint root"),
		core.RouteDescription("Proxies public HTTP GET requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_get"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_get"),
	)
	core.PostRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy POST to endpoint root"),
		core.RouteDescription("Proxies public HTTP POST requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_post"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_post"),
	)
	core.PutRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy PUT to endpoint root"),
		core.RouteDescription("Proxies public HTTP PUT requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_put"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_put"),
	)
	core.DeleteRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy DELETE to endpoint root"),
		core.RouteDescription("Proxies public HTTP DELETE requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_delete"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_delete"),
	)
	core.PatchRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy PATCH to endpoint root"),
		core.RouteDescription("Proxies public HTTP PATCH requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_patch"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_patch"),
	)
	core.HeadRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteHide(),
		core.RouteTag("Proxy"),
		core.RouteSummary("Reverse proxy HEAD to endpoint root"),
		core.RouteDescription("Proxies public HTTP HEAD requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_head"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_head"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Dynamic Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/function/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Get runtime function configuration"),
		core.RouteDescription("Returns operational function runtime settings including default isolate timeouts, memory limits, runner concurrency, and isolate pool capacities. Requires function:config.read scope."),
		core.RouteOperationID("function__config__get"),
		core.RouteSDKGroupName("function", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/function/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update runtime function configuration"),
		core.RouteDescription("Modifies operational function runtime settings in function.config, tuning concurrency limits, memory caps, and execution timeouts without restarting the gateway. Requires function:config.write scope."),
		core.RouteOperationID("function__config__update"),
		core.RouteSDKGroupName("function", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Endpoints Management
	core.GetRoute[ListEndpointsResponse](router, "/v1/_/function/endpoints", service.controlPlaneHandler.handleListEndpoints,
		core.RouteTag("Endpoints"),
		core.RouteSummary("List all registered serverless endpoints"),
		core.RouteDescription("Returns all registered serverless endpoints from function.endpoints with route names, runtime engines, timeout guards, memory limits, active deployment IDs, and status. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__endpoints__list"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Endpoint, CreateEndpointInput](router, "/v1/_/function/endpoints", service.controlPlaneHandler.handleCreateEndpoint,
		core.RouteTag("Endpoints"),
		core.RouteSummary("Register a new serverless endpoint"),
		core.RouteDescription("Provisions a new serverless endpoint definition in function.endpoints with route path, name, runtime environment (Node, Deno, Python, Go), timeout guards, memory limits, and environment variables. Emits function.endpoint.created event. Requires function:endpoint.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__endpoints__create"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Endpoint](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleGetEndpoint,
		core.RouteTag("Endpoints"),
		core.RouteSummary("Get serverless endpoint definition by ID"),
		core.RouteDescription("Retrieves complete definition of a serverless endpoint from function.endpoints by ID or name, including runtime configuration, environment variables, and active deployment version. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__endpoints__get"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Endpoint, UpdateEndpointInput](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleUpdateEndpoint,
		core.RouteTag("Endpoints"),
		core.RouteSummary("Update serverless endpoint definition"),
		core.RouteDescription("Modifies an existing serverless endpoint definition in function.endpoints, updating environment variables, timeout guards, or memory limits. Emits function.endpoint.updated event. Requires function:endpoint.write scope."),
		core.RouteOperationID("function__endpoints__update"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleDeleteEndpoint,
		core.RouteTag("Endpoints"),
		core.RouteSummary("Delete serverless endpoint and unload runtime isolates"),
		core.RouteDescription("Permanently deletes an endpoint from function.endpoints, unloads running worker isolates, cascades historical deployments, and removes custom domain routes. Emits function.endpoint.deleted event. Requires function:endpoint.write scope."),
		core.RouteNoContentResponse("Endpoint deleted"),
		core.RouteOperationID("function__endpoints__delete"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Deployments
	core.PostRoute[Deployment, CreateDeploymentInput](router, "/v1/_/function/endpoints/{endpoint_id}/deploy", service.controlPlaneHandler.handleCreateDeployment,
		core.RouteTag("Deployments"),
		core.RouteSummary("Create immutable deployment and hot-reload runner"),
		core.RouteDescription("Uploads and registers an immutable code bundle in function.deployments, increments revision version, sets it as the active deployment pointer, and immediately hot-reloads runtime worker isolates. Emits function.deployment.created event. Requires function:endpoint.deploy scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__deployments__create"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListDeploymentsResponse](router, "/v1/_/function/endpoints/{endpoint_id}/deployments", service.controlPlaneHandler.handleListDeployments,
		core.RouteTag("Deployments"),
		core.RouteSummary("List immutable deployments for an endpoint"),
		core.RouteDescription("Returns version history of immutable code deployments for an endpoint from function.deployments, detailing revision numbers, creation timestamps, and active flags. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__deployments__list"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Deployment, RollbackDeploymentInput](router, "/v1/_/function/endpoints/{endpoint_id}/rollback", service.controlPlaneHandler.handleRollbackDeployment,
		core.RouteTag("Deployments"),
		core.RouteSummary("Rollback endpoint to a previous deployment revision"),
		core.RouteDescription("Atomically updates the active deployment pointer in function.endpoints to an earlier immutable deployment version, instantly hot-reloading runtime runner isolates. Emits function.deployment.rollback event. Requires function:endpoint.deploy scope."),
		core.RouteOperationID("function__deployments__rollback"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("rollback"),
	)

	// 4. Invocations & Execution Logs
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/function/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Executions"),
		core.RouteSummary("Query execution history logs across all endpoints"),
		core.RouteDescription("Queries execution telemetry logs from function.executions across all endpoints, returning status codes, execution duration in milliseconds, memory utilization, and stdout/stderr streams. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__executions__list"),
		core.RouteSDKGroupName("function", "executions"),
		core.RouteSDKMethodName("list"),
	)
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/function/endpoints/{endpoint_id}/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Endpoints"),
		core.RouteSummary("Query execution history logs for a specific endpoint"),
		core.RouteDescription("Returns historical invocation audit records from function.executions for a specific endpoint, detailing response codes, durations, memory usage, and execution log streams. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__endpoints__executions"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("executions"),
	)

	// 5. Statistics & Telemetry
	core.GetRoute[GetStatsResponse](router, "/v1/_/function/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Stats"),
		core.RouteSummary("Get function runtime telemetry and worker statistics"),
		core.RouteDescription("Returns real-time runtime statistics including active worker isolates count, memory consumption, runner queue depth, and throughput metrics. Requires function:stats.read scope."),
		core.RouteOperationID("function__stats__get"),
		core.RouteSDKGroupName("function", "stats"),
		core.RouteSDKMethodName("get"),
	)

	// 6. Standalone Custom Domains
	core.PostRoute[CustomDomain, CreateCustomDomainInput](router, "/v1/_/function/domains", service.controlPlaneHandler.handleCreateCustomDomain,
		core.RouteTag("Domains"),
		core.RouteSummary("Register a standalone custom domain"),
		core.RouteDescription("Registers a standalone custom domain hostname in function.custom_domains with verification status and TLS configuration. Emits function.domain.created event. Requires function:endpoint.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__domains__create"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListCustomDomainsResponse](router, "/v1/_/function/domains", service.controlPlaneHandler.handleListCustomDomains,
		core.RouteTag("Domains"),
		core.RouteSummary("List registered custom domains"),
		core.RouteDescription("Returns all registered custom domain definitions from function.custom_domains with hostnames and verification states. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__domains__list"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/domains/{domain_id}", service.controlPlaneHandler.handleDeleteCustomDomain,
		core.RouteTag("Domains"),
		core.RouteSummary("Delete custom domain and cascade attached routes"),
		core.RouteDescription("Deletes a custom domain from function.custom_domains, cascading removal of all attached endpoint route mappings. Emits function.domain.deleted event. Requires function:endpoint.write scope."),
		core.RouteNoContentResponse("Custom domain deleted"),
		core.RouteOperationID("function__domains__delete"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("delete"),
	)

	// 7. Custom Domain Route Mappings
	core.PostRoute[CustomDomainRoute, CreateCustomDomainRouteInput](router, "/v1/_/function/endpoints/{endpoint_id}/domains", service.controlPlaneHandler.handleCreateCustomDomainRoute,
		core.RouteTag("Endpoint Domains"),
		core.RouteSummary("Attach endpoint route to custom domain with path prefix"),
		core.RouteDescription("Maps a custom domain hostname and URL path prefix to a specific serverless endpoint in function.custom_domain_routes, enabling custom domain routing. Requires function:endpoint.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__endpoint_domains__create"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListCustomDomainRoutesResponse](router, "/v1/_/function/endpoints/{endpoint_id}/domains", service.controlPlaneHandler.handleListCustomDomainRoutes,
		core.RouteTag("Endpoint Domains"),
		core.RouteSummary("List custom domain routes attached to an endpoint"),
		core.RouteDescription("Returns all custom domain route mappings from function.custom_domain_routes configured for a specific serverless endpoint. Requires function:endpoint.read scope."),
		core.RouteOperationID("function__endpoint_domains__list"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/endpoints/{endpoint_id}/domains/{route_id}", service.controlPlaneHandler.handleDeleteCustomDomainRoute,
		core.RouteTag("Endpoint Domains"),
		core.RouteSummary("Remove endpoint route mapping from custom domain"),
		core.RouteDescription("Removes a specific domain route mapping from function.custom_domain_routes, unbinding the custom path prefix from the endpoint. Requires function:endpoint.write scope."),
		core.RouteNoContentResponse("Custom domain route deleted"),
		core.RouteOperationID("function__endpoint_domains__delete"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("delete"),
	)
}
