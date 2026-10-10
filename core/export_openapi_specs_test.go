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
func (service *dummyServiceWithRegistrar) RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router) {
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
func (service *dummyServiceWithoutRegistrar) RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router) {
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

	openAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpecs := ExportOpenAPISpecs()
	require.NotNil(t, openAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, unifiedOpenAPISpecs)

	assert.NotEmpty(t, openAPISpec.Paths.Map())
	assert.NotEmpty(t, controlPlaneOpenAPISpec.Paths.Map())
	assert.NotEmpty(t, unifiedOpenAPISpecs.Paths.Map())

	// Public manifest route should be present in public and unified
	assert.NotNil(t, openAPISpec.Paths.Find("/v1/manifest"))
	assert.NotNil(t, unifiedOpenAPISpecs.Paths.Find("/v1/manifest"))
	assert.Nil(t, controlPlaneOpenAPISpec.Paths.Find("/v1/manifest"))

	// Hidden probe route should be absent from OpenAPI specs
	assert.Nil(t, openAPISpec.Paths.Find("/healthz"))
	assert.Nil(t, unifiedOpenAPISpecs.Paths.Find("/healthz"))

	// Control plane route should be present in control and unified
	assert.NotNil(t, controlPlaneOpenAPISpec.Paths.Find("/v1/_/core/service-accounts"))
	assert.NotNil(t, unifiedOpenAPISpecs.Paths.Find("/v1/_/core/service-accounts"))
	assert.Nil(t, openAPISpec.Paths.Find("/v1/_/core/service-accounts"))

	// Dummy route registered via mock registrar
	assert.NotNil(t, openAPISpec.Paths.Find("/v1/dummy/test"))
}

func TestCoreExportOpenAPISpecsMergeEdgeCasesUnit(t *testing.T) {
	// 1. Both nil
	mergedNilOpenAPISpecs := MergeOpenAPISpecs(nil, nil)
	require.NotNil(t, mergedNilOpenAPISpecs)
	assert.Equal(t, "3.1.0", mergedNilOpenAPISpecs.OpenAPI)

	// 2. Only public with custom openapi version and empty schemas
	openAPISpec := &openapi3.T{
		OpenAPI: "3.1.0",
		Paths:   openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"TestSchema": &openapi3.SchemaRef{
					Value: openapi3.NewStringSchema(),
				},
			},
		},
	}
	openAPISpec.Paths.Set("/test", &openapi3.PathItem{})

	mergedOneOpenAPISpec := MergeOpenAPISpecs(openAPISpec, nil)
	require.NotNil(t, mergedOneOpenAPISpec)
	assert.NotNil(t, mergedOneOpenAPISpec.Paths.Find("/test"))
	assert.NotNil(t, mergedOneOpenAPISpec.Components.Schemas["TestSchema"])

	// 3. Only control plane with paths and schemas
	controlPlaneOpenAPISpec := &openapi3.T{
		OpenAPI: "3.1.0",
		Paths:   openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"ControlSchema": &openapi3.SchemaRef{
					Value: openapi3.NewIntegerSchema(),
				},
			},
		},
	}
	controlPlaneOpenAPISpec.Paths.Set("/control", &openapi3.PathItem{})

	mergedOpenAPISpecs := MergeOpenAPISpecs(nil, controlPlaneOpenAPISpec)
	require.NotNil(t, mergedOpenAPISpecs)
	assert.NotNil(t, mergedOpenAPISpecs.Paths.Find("/control"))
	assert.NotNil(t, mergedOpenAPISpecs.Components.Schemas["ControlSchema"])
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
