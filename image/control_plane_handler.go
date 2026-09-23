package image

// ControlPlaneHandler coordinates all administrative control plane HTTP routes under /v1/_/image/*.
type ControlPlaneHandler struct {
	*Service
}

// NewControlPlaneHandler creates a new ControlPlaneHandler instance extending Service.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}
