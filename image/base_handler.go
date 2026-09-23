package image

// BaseHandler coordinates all public image data plane HTTP routes.
type BaseHandler struct {
	*Service
}

// NewBaseHandler initializes the BaseHandler extending Service.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}
