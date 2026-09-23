// Package tasks provides distributed cron and background task orchestration.
package tasks

// ControlPlaneHandler coordinates all administrative control plane HTTP routes under /v1/_/tasks/*.
type ControlPlaneHandler struct {
	*Service
}

// NewControlPlaneHandler creates a new ControlPlaneHandler instance extending Service.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}
