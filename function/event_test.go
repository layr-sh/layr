package function

import (
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
)

func TestFunctionEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("config updated event", func(t *testing.T) {
		t.Parallel()
		configUpdatedEventData := ConfigUpdatedEventData(DefaultConfig())
		event := NewConfigUpdatedEvent("function.config", configUpdatedEventData)
		require.Equal(t, "function.config.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "function.config", *event.ResourceID)
	})

	t.Run("endpoint created event", func(t *testing.T) {
		t.Parallel()
		endpointID := uuid.NewV7()
		endpointCreatedEventData := EndpointCreatedEventData(Endpoint{
			ID:        endpointID,
			Name:      "hello",
			Runtime:   "workerd",
			CreatedAt: time.Now().UTC(),
		})
		event := NewEndpointCreatedEvent(endpointID.String(), endpointCreatedEventData)
		require.Equal(t, "function.endpoint.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, endpointID.String(), *event.ResourceID)
	})

	t.Run("endpoint updated event", func(t *testing.T) {
		t.Parallel()
		endpointID := uuid.NewV7()
		endpointUpdatedEventData := EndpointUpdatedEventData(Endpoint{
			ID:        endpointID,
			Name:      "hello-v2",
			Runtime:   "workerd",
			UpdatedAt: time.Now().UTC(),
		})
		event := NewEndpointUpdatedEvent(endpointID.String(), endpointUpdatedEventData)
		require.Equal(t, "function.endpoint.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, endpointID.String(), *event.ResourceID)
	})

	t.Run("endpoint deleted event", func(t *testing.T) {
		t.Parallel()
		endpointID := uuid.NewV7()
		endpointDeletedEventData := EndpointDeletedEventData(Endpoint{
			ID:   endpointID,
			Name: "hello",
		})
		event := NewEndpointDeletedEvent(endpointID.String(), endpointDeletedEventData)
		require.Equal(t, "function.endpoint.deleted", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, endpointID.String(), *event.ResourceID)
	})

	t.Run("deployment created event", func(t *testing.T) {
		t.Parallel()
		deploymentID := uuid.NewV7()
		deploymentCreatedEventData := DeploymentCreatedEventData(Deployment{
			ID:         deploymentID,
			EndpointID: uuid.NewV7(),
			Version:    1,
			BundleHash: "sha256-abc",
			CreatedAt:  time.Now().UTC(),
		})
		event := NewDeploymentCreatedEvent(deploymentID.String(), deploymentCreatedEventData)
		require.Equal(t, "function.deployment.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, deploymentID.String(), *event.ResourceID)
	})

	t.Run("deployment rolled back event", func(t *testing.T) {
		t.Parallel()
		deploymentID := uuid.NewV7()
		endpointID := uuid.NewV7()
		deploymentRolledBackEventData := DeploymentRolledBackEventData(Deployment{
			ID:         deploymentID,
			EndpointID: endpointID,
			Version:    1,
		})
		event := NewDeploymentRolledBackEvent(deploymentID.String(), deploymentRolledBackEventData)
		require.Equal(t, "function.deployment.rolled_back", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, deploymentID.String(), *event.ResourceID)
	})

	t.Run("execution completed event", func(t *testing.T) {
		t.Parallel()
		endpointID := uuid.NewV7()
		executionID := uuid.NewV7()
		executionCompletedEventData := ExecutionCompletedEventData(Execution{
			ID:         executionID,
			EndpointID: endpointID,
			StatusCode: 200,
			DurationMs: 12,
		})
		event := NewExecutionCompletedEvent(executionID.String(), executionCompletedEventData)
		require.Equal(t, "function.execution.completed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, executionID.String(), *event.ResourceID)
	})

	t.Run("execution failed event", func(t *testing.T) {
		t.Parallel()
		endpointID := uuid.NewV7()
		executionID := uuid.NewV7()
		errMsg := "out of memory"
		executionFailedEventData := ExecutionFailedEventData(Execution{
			ID:           executionID,
			EndpointID:   endpointID,
			StatusCode:   502,
			DurationMs:   45,
			ErrorMessage: &errMsg,
		})
		event := NewExecutionFailedEvent(executionID.String(), executionFailedEventData)
		require.Equal(t, "function.execution.failed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, executionID.String(), *event.ResourceID)
	})

	t.Run("custom domain created event", func(t *testing.T) {
		t.Parallel()
		domainID := uuid.NewV7()
		customDomainCreatedEventData := CustomDomainCreatedEventData(CustomDomain{
			ID:     domainID,
			Domain: "api.example.com",
			Status: "active",
		})
		event := NewCustomDomainCreatedEvent(domainID.String(), customDomainCreatedEventData)
		require.Equal(t, "function.custom_domain.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, domainID.String(), *event.ResourceID)
	})

	t.Run("custom domain deleted event", func(t *testing.T) {
		t.Parallel()
		domainID := uuid.NewV7()
		customDomainDeletedEventData := CustomDomainDeletedEventData(CustomDomain{
			ID:     domainID,
			Domain: "api.example.com",
		})
		event := NewCustomDomainDeletedEvent(domainID.String(), customDomainDeletedEventData)
		require.Equal(t, "function.custom_domain.deleted", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, domainID.String(), *event.ResourceID)
	})

	t.Run("custom domain route created event", func(t *testing.T) {
		t.Parallel()
		routeID := uuid.NewV7()
		customDomainRouteCreatedEventData := CustomDomainRouteCreatedEventData(CustomDomainRoute{
			ID:             routeID,
			CustomDomainID: uuid.NewV7(),
			EndpointID:     uuid.NewV7(),
			PathPrefix:     "/users",
		})
		event := NewCustomDomainRouteCreatedEvent(routeID.String(), customDomainRouteCreatedEventData)
		require.Equal(t, "function.custom_domain_route.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, routeID.String(), *event.ResourceID)
	})

	t.Run("custom domain route deleted event", func(t *testing.T) {
		t.Parallel()
		routeID := uuid.NewV7()
		domainID := uuid.NewV7()
		endpointID := uuid.NewV7()
		customDomainRouteDeletedEventData := CustomDomainRouteDeletedEventData(CustomDomainRoute{
			ID:             routeID,
			CustomDomainID: domainID,
			EndpointID:     endpointID,
			PathPrefix:     "/users",
		})
		event := NewCustomDomainRouteDeletedEvent(routeID.String(), customDomainRouteDeletedEventData)
		require.Equal(t, "function.custom_domain_route.deleted", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, routeID.String(), *event.ResourceID)
	})
}
