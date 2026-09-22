package auth

// ControlPlaneHandler exposes protected user and session management endpoints for the control plane.
type ControlPlaneHandler struct {
	*Service
}

// NewControlPlaneHandler creates a control plane handler for auth endpoints extending Service.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}
