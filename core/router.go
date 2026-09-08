package core

import (
	"net/http"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
)

// RouteOption alias for OpenAPI and routing customization
type RouteOption = func(*fuego.BaseRoute)

// RouteTag attaches OpenAPI tags to the route.
func RouteTag(tags ...string) RouteOption {
	return fuego.OptionTags(tags...)
}

// RouteSummary attaches an OpenAPI summary to the route.
func RouteSummary(summary string) RouteOption {
	return fuego.OptionSummary(summary)
}

// RouteDescription sets the OpenAPI route description, overriding the default internal controller header.
func RouteDescription(description string) RouteOption {
	return fuego.OptionOverrideDescription(description)
}

// RouteOperationID sets the OpenAPI operation ID for the route.
func RouteOperationID(operationID string) RouteOption {
	return fuego.OptionOperationID(operationID)
}

// RouteDefaultStatusCode sets the default HTTP status code for the route.
func RouteDefaultStatusCode(defaultStatusCode int) RouteOption {
	return fuego.OptionDefaultStatusCode(defaultStatusCode)
}

// RouteResponseHeader documents an expected response header for the route.
func RouteResponseHeader(name, description string) RouteOption {
	return fuego.OptionResponseHeader(name, description)
}

// RouteBinaryResponse configures an OpenAPI response for binary streaming data (images, files, octet-streams).
func RouteBinaryResponse(statusCode int, description string, contentTypes ...string) RouteOption {
	return func(route *fuego.BaseRoute) {
		if len(contentTypes) == 0 {
			contentTypes = []string{"application/octet-stream"}
		}
		schema := openapi3.NewStringSchema().WithFormat("binary")
		content := openapi3.NewContentWithSchema(schema, contentTypes)
		response := openapi3.NewResponse().
			WithDescription(description).
			WithContent(content)

		if route.Operation.Responses == nil {
			route.Operation.Responses = openapi3.NewResponses()
		}
		route.Operation.Responses.Set(strconv.Itoa(statusCode), &openapi3.ResponseRef{Value: response})
	}
}

// RouteNoContentResponse configures a 204 No Content response without content payload.
func RouteNoContentResponse(description string) RouteOption {
	return func(route *fuego.BaseRoute) {
		response := openapi3.NewResponse().WithDescription(description)
		if route.Operation.Responses == nil {
			route.Operation.Responses = openapi3.NewResponses()
		}
		route.Operation.Responses.Delete("200")
		route.Operation.Responses.Set(strconv.Itoa(http.StatusNoContent), &openapi3.ResponseRef{Value: response})
	}
}

// RouteNoRequestBody removes any generated request body from the route specification.
func RouteNoRequestBody() RouteOption {
	return func(route *fuego.BaseRoute) {
		route.Operation.RequestBody = nil
	}
}

// RouteResponseModel configures a custom response model and status code for an OpenAPI route.
func RouteResponseModel(statusCode int, description string, model any) RouteOption {
	return fuego.OptionAddResponse(statusCode, description, fuego.Response{Type: model})
}

// RouteRequestBodyModel configures a custom request body model for an OpenAPI route.
func RouteRequestBodyModel(model any, contentTypes ...string) RouteOption {
	return fuego.OptionRequestBody(fuego.RequestBody{Type: model, ContentTypes: contentTypes})
}

// RouteSDKGroupName sets the SDK subclient namespace hierarchy (e.g. "data", "cache" or "data", "control").
func RouteSDKGroupName(names ...string) RouteOption {
	return func(route *fuego.BaseRoute) {
		if route.Operation.Extensions == nil {
			route.Operation.Extensions = make(map[string]any)
		}
		route.Operation.Extensions["x-sdk-group-name"] = names
	}
}

// RouteSDKMethodName sets the explicit SDK method name on the generated client.
func RouteSDKMethodName(name string) RouteOption {
	return func(route *fuego.BaseRoute) {
		if route.Operation.Extensions == nil {
			route.Operation.Extensions = make(map[string]any)
		}
		route.Operation.Extensions["x-sdk-method-name"] = name
	}
}

// Router wraps the underlying OpenAPI and HTTP router engine.
type Router struct {
	engine *fuego.Server
}

type netHTTPRouteRegisterer[T, B, P any] struct {
	server     *fuego.Server
	controller http.Handler
	route      fuego.Route[T, B, P]
	options    []RouteOption
}

func (registerer netHTTPRouteRegisterer[T, B, P]) Register() fuego.Route[T, B, P] {
	route := registerer.route
	for _, option := range registerer.options {
		option(&route.BaseRoute)
	}

	fullPath := route.Path
	if route.Method != "" {
		fullPath = route.Method + " " + fullPath
	}

	log.Tracef("registering HTTP route: %s", fullPath)
	registerer.server.Mux.Handle(fullPath, registerer.controller)
	return route
}

// NewRouter creates a new Router instance wrapping a fuego.Server with standard schema generator settings.
func NewRouter(engine *fuego.Server) *Router {
	return &Router{engine: engine}
}

// GetRoute registers an http.HandlerFunc on GET with strongly-typed response type T for OpenAPI.
func GetRoute[T any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, any, any](http.MethodGet, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, any, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// HeadRoute registers an http.HandlerFunc on HEAD with strongly-typed response type T for OpenAPI.
func HeadRoute[T any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, any, any](http.MethodHead, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, any, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// PostRoute registers an http.HandlerFunc on POST with strongly-typed response type T and request body B for OpenAPI.
func PostRoute[T any, B any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, B, any](http.MethodPost, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, B, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// PutRoute registers an http.HandlerFunc on PUT with strongly-typed response type T and request body B for OpenAPI.
func PutRoute[T any, B any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, B, any](http.MethodPut, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, B, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// DeleteRoute registers an http.HandlerFunc on DELETE with strongly-typed response type T for OpenAPI.
func DeleteRoute[T any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, any, any](http.MethodDelete, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, any, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// PatchRoute registers an http.HandlerFunc on PATCH with strongly-typed response type T and request body B for OpenAPI.
func PatchRoute[T any, B any](router *Router, path string, handler func(http.ResponseWriter, *http.Request), options ...RouteOption) {
	route := fuego.NewRoute[T, B, any](http.MethodPatch, path, handler, router.engine.Engine, options...)
	fuego.Registers(router.engine.Engine, netHTTPRouteRegisterer[T, B, any]{
		server:     router.engine,
		route:      route,
		controller: http.HandlerFunc(handler),
		options:    options,
	})
}

// OutputOpenAPISpec returns the OpenAPI 3.1 specification.
func (router *Router) OutputOpenAPISpec() *openapi3.T {
	if router.engine != nil && router.engine.OpenAPI != nil {
		return router.engine.OpenAPI.Description()
	}
	return nil
}

// Engine returns the underlying fuego.Server engine.
func (router *Router) Engine() *fuego.Server {
	return router.engine
}

// Mux returns the underlying http.ServeMux.
func (router *Router) Mux() *http.ServeMux {
	return router.engine.Mux
}
