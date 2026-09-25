// Package function defines the serverless and edge function execution engine.
package function

// BaseHandler coordinates all public data plane HTTP routes for endpoint execution.
type BaseHandler struct {
	*Service
}

// NewBaseHandler initializes the BaseHandler extending Service.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}
