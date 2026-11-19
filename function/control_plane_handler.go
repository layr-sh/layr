package function

import (
	"context"
	"errors"
	"uuid"
)

// ControlPlaneHandler coordinates all administrative control plane HTTP routes under /v1/_/function/*.
type ControlPlaneHandler struct {
	*Service
}

// restoreEndpointRuntime reconciles a failed database mutation with the persisted active version.
func (controlPlaneHandler *ControlPlaneHandler) restoreEndpointRuntime(ctx context.Context, endpointID uuid.UUID) error {
	endpoint, fetchErr := controlPlaneHandler.fetchEndpointByID(ctx, endpointID)
	if fetchErr != nil {
		return fetchErr
	}
	deployment, deploymentErr := controlPlaneHandler.baseHandler.fetchActiveDeployment(ctx, endpointID)
	if errors.Is(deploymentErr, ErrNoActiveDeployment) {
		return controlPlaneHandler.engine.Undeploy(ctx, endpoint.Runtime, endpoint.Name)
	}
	if deploymentErr != nil {
		return deploymentErr
	}
	return controlPlaneHandler.engine.Deploy(ctx, endpoint, deployment)
}

// NewControlPlaneHandler creates a new ControlPlaneHandler instance extending Service.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}
