package function

import "layr.sh/core"

// ConfigUpdatedEventData represents the payload for function.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for function configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("function.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// EndpointCreatedEventData represents the payload for function.endpoint.created.
type EndpointCreatedEventData Endpoint

// NewEndpointCreatedEvent creates a typed event for endpoint definitions created.
func NewEndpointCreatedEvent(resourceID string, endpointCreatedEventData EndpointCreatedEventData) core.Event {
	return core.NewEvent("function.endpoint.created", endpointCreatedEventData).WithResourceID(resourceID)
}

// EndpointUpdatedEventData represents the payload for function.endpoint.updated.
type EndpointUpdatedEventData Endpoint

// NewEndpointUpdatedEvent creates a typed event for endpoint definitions updated.
func NewEndpointUpdatedEvent(resourceID string, endpointUpdatedEventData EndpointUpdatedEventData) core.Event {
	return core.NewEvent("function.endpoint.updated", endpointUpdatedEventData).WithResourceID(resourceID)
}

// EndpointDeletedEventData represents the payload for function.endpoint.deleted.
type EndpointDeletedEventData Endpoint

// NewEndpointDeletedEvent creates a typed event for endpoint definitions deleted.
func NewEndpointDeletedEvent(resourceID string, endpointDeletedEventData EndpointDeletedEventData) core.Event {
	return core.NewEvent("function.endpoint.deleted", endpointDeletedEventData).WithResourceID(resourceID)
}

// DeploymentCreatedEventData represents the payload for function.deployment.created.
type DeploymentCreatedEventData Deployment

// NewDeploymentCreatedEvent creates a typed event for endpoint deployments created.
func NewDeploymentCreatedEvent(resourceID string, deploymentCreatedEventData DeploymentCreatedEventData) core.Event {
	return core.NewEvent("function.deployment.created", deploymentCreatedEventData).WithResourceID(resourceID)
}

// DeploymentRolledBackEventData represents the payload for function.deployment.rolled_back.
type DeploymentRolledBackEventData Deployment

// NewDeploymentRolledBackEvent creates a typed event for endpoint deployment rollbacks.
func NewDeploymentRolledBackEvent(resourceID string, deploymentRolledBackEventData DeploymentRolledBackEventData) core.Event {
	return core.NewEvent("function.deployment.rolled_back", deploymentRolledBackEventData).WithResourceID(resourceID)
}

// ExecutionCompletedEventData represents the payload for function.execution.completed.
type ExecutionCompletedEventData Execution

// NewExecutionCompletedEvent creates a typed event for successful endpoint executions.
func NewExecutionCompletedEvent(resourceID string, executionCompletedEventData ExecutionCompletedEventData) core.Event {
	return core.NewEvent("function.execution.completed", executionCompletedEventData).WithResourceID(resourceID)
}

// ExecutionFailedEventData represents the payload for function.execution.failed.
type ExecutionFailedEventData Execution

// NewExecutionFailedEvent creates a typed event for failed endpoint executions.
func NewExecutionFailedEvent(resourceID string, executionFailedEventData ExecutionFailedEventData) core.Event {
	return core.NewEvent("function.execution.failed", executionFailedEventData).WithResourceID(resourceID)
}

// CustomDomainCreatedEventData represents the payload for function.custom_domain.created.
type CustomDomainCreatedEventData CustomDomain

// NewCustomDomainCreatedEvent creates a typed event for custom domain creation.
func NewCustomDomainCreatedEvent(resourceID string, customDomainCreatedEventData CustomDomainCreatedEventData) core.Event {
	return core.NewEvent("function.custom_domain.created", customDomainCreatedEventData).WithResourceID(resourceID)
}

// CustomDomainDeletedEventData represents the payload for function.custom_domain.deleted.
type CustomDomainDeletedEventData CustomDomain

// NewCustomDomainDeletedEvent creates a typed event for custom domain deletion.
func NewCustomDomainDeletedEvent(resourceID string, customDomainDeletedEventData CustomDomainDeletedEventData) core.Event {
	return core.NewEvent("function.custom_domain.deleted", customDomainDeletedEventData).WithResourceID(resourceID)
}

// CustomDomainRouteCreatedEventData represents the payload for function.custom_domain_route.created.
type CustomDomainRouteCreatedEventData CustomDomainRoute

// NewCustomDomainRouteCreatedEvent creates a typed event for custom domain route creation.
func NewCustomDomainRouteCreatedEvent(resourceID string, customDomainRouteCreatedEventData CustomDomainRouteCreatedEventData) core.Event {
	return core.NewEvent("function.custom_domain_route.created", customDomainRouteCreatedEventData).WithResourceID(resourceID)
}

// CustomDomainRouteDeletedEventData represents the payload for function.custom_domain_route.deleted.
type CustomDomainRouteDeletedEventData CustomDomainRoute

// NewCustomDomainRouteDeletedEvent creates a typed event for custom domain route deletion.
func NewCustomDomainRouteDeletedEvent(resourceID string, customDomainRouteDeletedEventData CustomDomainRouteDeletedEventData) core.Event {
	return core.NewEvent("function.custom_domain_route.deleted", customDomainRouteDeletedEventData).WithResourceID(resourceID)
}
