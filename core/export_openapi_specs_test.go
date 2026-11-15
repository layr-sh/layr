package core

import (
	"context"
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyServiceWithRegistrar struct{}

func (service *dummyServiceWithRegistrar) Start(ctx context.Context) error { return nil }
func (service *dummyServiceWithRegistrar) Stop()                           {}
func (service *dummyServiceWithRegistrar) RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router, gatewayServeMux *http.ServeMux) {
	GetRoute[string](baseRouter, "/v1/dummy/test", func(responseWriter http.ResponseWriter, request *http.Request) {},
		RouteTag("Dummy"),
		RouteSummary("Dummy route"),
		RouteOperationID("dummy__get"),
		RouteSDKGroupName("dummy"),
		RouteSDKMethodName("get"),
	)
}

type dummyServiceWithoutRegistrar struct{}

func (service *dummyServiceWithoutRegistrar) Start(ctx context.Context) error { return nil }
func (service *dummyServiceWithoutRegistrar) Stop()                           {}
func (service *dummyServiceWithoutRegistrar) RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router, gatewayServeMux *http.ServeMux) {
}

func TestCoreExportOpenAPISpecsUnit(t *testing.T) {
	RegisterServiceFactory("test_dummy_registrar", func(kernel *Kernel) (ServiceRunner, error) {
		return &dummyServiceWithRegistrar{}, nil
	})
	RegisterServiceFactory("test_dummy_nonregistrar", func(kernel *Kernel) (ServiceRunner, error) {
		return &dummyServiceWithoutRegistrar{}, nil
	})
	RegisterServiceFactory("test_dummy_error", func(kernel *Kernel) (ServiceRunner, error) {
		return nil, assert.AnError
	})
	RegisterServiceFactory("test_dummy_registrar_dup", func(kernel *Kernel) (ServiceRunner, error) {
		return &dummyServiceWithRegistrar{}, nil
	})

	baseOpenAPISpecs, controlPlaneOpenAPISpecs := ExportOpenAPISpecs()
	require.NotNil(t, baseOpenAPISpecs["core"])
	require.NotNil(t, controlPlaneOpenAPISpecs["core"])
	assert.NotNil(t, baseOpenAPISpecs["core"].Paths.Find("/v1/manifest"))
	assert.Nil(t, controlPlaneOpenAPISpecs["core"].Paths.Find("/v1/manifest"))
	assert.Nil(t, baseOpenAPISpecs["core"].Paths.Find("/healthz"))
	assert.Nil(t, baseOpenAPISpecs["core"].Paths.Find("/v1/spec.json"))
	assert.Nil(t, controlPlaneOpenAPISpecs["core"].Paths.Find("/v1/_/spec.json"))
	assert.NotNil(t, controlPlaneOpenAPISpecs["core"].Paths.Find("/v1/_/core/service-accounts"))
	assert.Nil(t, baseOpenAPISpecs["core"].Paths.Find("/v1/_/core/service-accounts"))

	require.NotNil(t, baseOpenAPISpecs["test_dummy_registrar"])
	assert.NotNil(t, baseOpenAPISpecs["test_dummy_registrar"].Paths.Find("/v1/dummy/test"))
	assert.Nil(t, baseOpenAPISpecs["test_dummy_registrar"].Paths.Find("/v1/manifest"))
	assert.Nil(t, baseOpenAPISpecs["core"].Paths.Find("/v1/dummy/test"))
	assert.NotContains(t, baseOpenAPISpecs, "test_dummy_registrar_dup")
	assert.NotContains(t, baseOpenAPISpecs, "test_dummy_error")
}

func TestCoreSanitizeOpenAPISpecUnit(t *testing.T) {
	// 1. Nil spec and nil schema
	sanitizeOpenAPISpec(nil)
	sanitizeSchemaValue(nil)

	// 2. Spec with standalone "null" type in components and properties
	testOpenAPISpec := &openapi3.T{
		OpenAPI: "3.1.0",
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"": &openapi3.SchemaRef{Value: openapi3.NewObjectSchema()},
				"NullSchema": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"null"},
					},
				},
				"ParentSchema": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Properties: openapi3.Schemas{
							"child": &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"null"},
								},
							},
						},
					},
				},
			},
		},
		Paths: openapi3.NewPaths(),
	}

	// 3. Paths with nil path item, broken ref in request body and responses
	brokenRequestBodyRef := &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Ref: "#/components/schemas/",
					},
				},
			},
		},
	}
	brokenResponseRef := &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Ref: "#/components/schemas/",
					},
				},
			},
		},
	}
	responses := openapi3.NewResponses()
	responses.Set("200", brokenResponseRef)

	testOpenAPISpec.Paths.Set("/nil-item", nil)
	testOpenAPISpec.Paths.Set("/test", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: brokenRequestBodyRef,
			Responses:   responses,
		},
	})

	sanitizeOpenAPISpec(testOpenAPISpec)
	assert.NotContains(t, testOpenAPISpec.Components.Schemas, "")
	assert.Equal(t, &openapi3.Types{"object"}, testOpenAPISpec.Components.Schemas["NullSchema"].Value.Type)
	assert.Empty(t, brokenRequestBodyRef.Value.Content["application/json"].Schema.Ref)
	assert.Empty(t, brokenResponseRef.Value.Content["application/json"].Schema.Ref)
}
