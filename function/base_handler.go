package function

// BaseHandler coordinates all public data plane HTTP routes for endpoint execution.
type BaseHandler struct {
	*Service
}

// NewBaseHandler creates a new BaseHandler instance extending Service.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}
