// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

// ControlPlaneHandler coordinates administrative HTTP endpoints under /v1/_/console/*.
type ControlPlaneHandler struct {
	*Service
}

// NewControlPlaneHandler creates a new ControlPlaneHandler instance extending Service.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}
