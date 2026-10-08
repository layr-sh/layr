package image

// BaseHandler coordinates all public image data plane HTTP routes.
type BaseHandler struct {
	*Service
}

// NewBaseHandler creates a new BaseHandler instance extending Service.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}
