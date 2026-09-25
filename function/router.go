// Package function defines the serverless and edge function execution engine.
package function

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Function data plane and administrative control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	if service.kernel != nil && service.kernel.Server() != nil {
		service.RegisterHostRoutes(service.kernel.Server())
	}
	if baseRouter != nil {
		service.registerBaseRoutes(baseRouter)
	}
	if controlPlaneRouter != nil {
		service.registerControlPlaneRoutes(controlPlaneRouter)
	}
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// Proxy routes for subpaths
	core.GetRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy GET to endpoint"),
		core.RouteDescription("Proxies public HTTP GET requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__get"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy POST to endpoint"),
		core.RouteDescription("Proxies public HTTP POST requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__post"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("post"),
	)
	core.PutRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy PUT to endpoint"),
		core.RouteDescription("Proxies public HTTP PUT requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__put"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("put"),
	)
	core.DeleteRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy DELETE to endpoint"),
		core.RouteDescription("Proxies public HTTP DELETE requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__delete"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("delete"),
	)
	core.PatchRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy PATCH to endpoint"),
		core.RouteDescription("Proxies public HTTP PATCH requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__patch"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("patch"),
	)
	core.HeadRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}/{path...}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy HEAD to endpoint"),
		core.RouteDescription("Proxies public HTTP HEAD requests directly to the active endpoint isolate."),
		core.RouteOperationID("function__proxy__head"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("head"),
	)

	// Proxy routes for root endpoint path
	core.GetRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy GET to endpoint root"),
		core.RouteDescription("Proxies public HTTP GET requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_get"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_get"),
	)
	core.PostRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy POST to endpoint root"),
		core.RouteDescription("Proxies public HTTP POST requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_post"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_post"),
	)
	core.PutRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy PUT to endpoint root"),
		core.RouteDescription("Proxies public HTTP PUT requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_put"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_put"),
	)
	core.DeleteRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy DELETE to endpoint root"),
		core.RouteDescription("Proxies public HTTP DELETE requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_delete"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_delete"),
	)
	core.PatchRoute[InvokeEndpointResponse, InvokeEndpointInput](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
		core.RouteSummary("Reverse proxy PATCH to endpoint root"),
		core.RouteDescription("Proxies public HTTP PATCH requests directly to the active endpoint root."),
		core.RouteOperationID("function__proxy__root_patch"),
		core.RouteSDKGroupName("function", "proxy"),
		core.RouteSDKMethodName("root_patch"),
	)
	core.HeadRoute[InvokeEndpointResponse](router, "/v1/function/{endpoint_name}", service.baseHandler.handleInvokeEndpoint,
		core.RouteTag("Function Data Plane"),
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
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Get runtime configuration"),
		core.RouteDescription("Returns the dynamic operational configuration for the function runtime subsystem."),
		core.RouteOperationID("function__config__get"),
		core.RouteSDKGroupName("function", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/function/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Update runtime configuration"),
		core.RouteDescription("Modifies dynamic operational configuration settings for the function runtime subsystem."),
		core.RouteOperationID("function__config__update"),
		core.RouteSDKGroupName("function", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Endpoints Management
	core.GetRoute[ListEndpointsResponse](router, "/v1/_/function/endpoints", service.controlPlaneHandler.handleListEndpoints,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List endpoints"),
		core.RouteDescription("Returns all registered serverless endpoints."),
		core.RouteOperationID("function__endpoints__list"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Endpoint, CreateEndpointInput](router, "/v1/_/function/endpoints", service.controlPlaneHandler.handleCreateEndpoint,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Create endpoint"),
		core.RouteDescription("Registers a new serverless endpoint definition."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__endpoints__create"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Endpoint](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleGetEndpoint,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Get endpoint"),
		core.RouteDescription("Returns details of a single serverless endpoint definition."),
		core.RouteOperationID("function__endpoints__get"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Endpoint, UpdateEndpointInput](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleUpdateEndpoint,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Update endpoint"),
		core.RouteDescription("Modifies an existing serverless endpoint definition."),
		core.RouteOperationID("function__endpoints__update"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/endpoints/{endpoint_id}", service.controlPlaneHandler.handleDeleteEndpoint,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Delete endpoint"),
		core.RouteDescription("Permanently removes an endpoint definition and unloads it from running runtimes."),
		core.RouteNoContentResponse("Endpoint deleted"),
		core.RouteOperationID("function__endpoints__delete"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Deployments
	core.PostRoute[Deployment, CreateDeploymentInput](router, "/v1/_/function/endpoints/{endpoint_id}/deploy", service.controlPlaneHandler.handleCreateDeployment,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Deploy endpoint"),
		core.RouteDescription("Uploads an immutable code bundle and hot-reloads it in the active runtime runner."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__deployments__create"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListDeploymentsResponse](router, "/v1/_/function/endpoints/{endpoint_id}/deployments", service.controlPlaneHandler.handleListDeployments,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List deployments"),
		core.RouteDescription("Returns immutable deployment version history for an endpoint."),
		core.RouteOperationID("function__deployments__list"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Deployment, RollbackDeploymentInput](router, "/v1/_/function/endpoints/{endpoint_id}/rollback", service.controlPlaneHandler.handleRollbackDeployment,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Rollback deployment"),
		core.RouteDescription("Restores an earlier immutable deployment version as active."),
		core.RouteOperationID("function__deployments__rollback"),
		core.RouteSDKGroupName("function", "deployments"),
		core.RouteSDKMethodName("rollback"),
	)

	// 4. Invocations & Execution Logs
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/function/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List execution history logs"),
		core.RouteDescription("Returns historical invocation logs including status codes, duration, stdout, and stderr."),
		core.RouteOperationID("function__executions__list"),
		core.RouteSDKGroupName("function", "executions"),
		core.RouteSDKMethodName("list"),
	)
	core.GetRoute[ListExecutionsResponse](router, "/v1/_/function/endpoints/{endpoint_id}/executions", service.controlPlaneHandler.handleListExecutions,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List endpoint execution logs"),
		core.RouteDescription("Returns historical invocation logs for a specific endpoint including status codes, duration, stdout, and stderr."),
		core.RouteOperationID("function__endpoints__executions"),
		core.RouteSDKGroupName("function", "endpoints"),
		core.RouteSDKMethodName("executions"),
	)

	// 5. Statistics & Telemetry
	core.GetRoute[GetStatsResponse](router, "/v1/_/function/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Get runtime statistics"),
		core.RouteDescription("Returns telemetry, active isolates count, memory usage, and runner statuses."),
		core.RouteOperationID("function__stats__get"),
		core.RouteSDKGroupName("function", "stats"),
		core.RouteSDKMethodName("get"),
	)

	// 6. Standalone Custom Domains
	core.PostRoute[CustomDomain, CreateCustomDomainInput](router, "/v1/_/function/domains", service.controlPlaneHandler.handleCreateCustomDomain,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Create custom domain"),
		core.RouteDescription("Registers a standalone custom domain hostname."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__domains__create"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListCustomDomainsResponse](router, "/v1/_/function/domains", service.controlPlaneHandler.handleListCustomDomains,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List custom domains"),
		core.RouteDescription("Returns all registered custom domains."),
		core.RouteOperationID("function__domains__list"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/domains/{domain_id}", service.controlPlaneHandler.handleDeleteCustomDomain,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Delete custom domain"),
		core.RouteDescription("Removes a registered custom domain and cascades its routes."),
		core.RouteNoContentResponse("Custom domain deleted"),
		core.RouteOperationID("function__domains__delete"),
		core.RouteSDKGroupName("function", "domains"),
		core.RouteSDKMethodName("delete"),
	)

	// 7. Custom Domain Route Mappings
	core.PostRoute[CustomDomainRoute, CreateCustomDomainRouteInput](router, "/v1/_/function/endpoints/{endpoint_id}/domains", service.controlPlaneHandler.handleCreateCustomDomainRoute,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Create custom domain route"),
		core.RouteDescription("Attaches an endpoint to a custom domain route with a path prefix."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("function__endpoint_domains__create"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListCustomDomainRoutesResponse](router, "/v1/_/function/endpoints/{endpoint_id}/domains", service.controlPlaneHandler.handleListCustomDomainRoutes,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("List custom domain routes"),
		core.RouteDescription("Returns all custom domain routes attached to an endpoint."),
		core.RouteOperationID("function__endpoint_domains__list"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/function/endpoints/{endpoint_id}/domains/{route_id}", service.controlPlaneHandler.handleDeleteCustomDomainRoute,
		core.RouteTag("Function Control Plane"),
		core.RouteSummary("Delete custom domain route"),
		core.RouteDescription("Removes an endpoint route mapping from a custom domain."),
		core.RouteNoContentResponse("Custom domain route deleted"),
		core.RouteOperationID("function__endpoint_domains__delete"),
		core.RouteSDKGroupName("function", "endpoint_domains"),
		core.RouteSDKMethodName("delete"),
	)
}
