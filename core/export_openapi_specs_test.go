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
func (service *dummyServiceWithRegistrar) Stop() error                     { return nil }
func (service *dummyServiceWithRegistrar) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
	GetRoute[string](router, "/v1/dummy/test", func(responseWriter http.ResponseWriter, request *http.Request) {},
		RouteTag("Dummy"),
		RouteSummary("Dummy route"),
		RouteOperationID("getDummy"),
	)
}

type dummyServiceWithoutRegistrar struct{}

func (service *dummyServiceWithoutRegistrar) Start(ctx context.Context) error { return nil }
func (service *dummyServiceWithoutRegistrar) Stop() error                     { return nil }
func (service *dummyServiceWithoutRegistrar) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
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

	openAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpecs, err := ExportOpenAPISpecs()
	require.NoError(t, err)
	require.NotNil(t, openAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, unifiedOpenAPISpecs)

	assert.NotEmpty(t, openAPISpec.Paths.Map())
	assert.NotEmpty(t, controlPlaneOpenAPISpec.Paths.Map())
	assert.NotEmpty(t, unifiedOpenAPISpecs.Paths.Map())

	// Public probe route should be present in public and unified
	assert.NotNil(t, openAPISpec.Paths.Find("/healthz"))
	assert.NotNil(t, unifiedOpenAPISpecs.Paths.Find("/healthz"))
	assert.Nil(t, controlPlaneOpenAPISpec.Paths.Find("/healthz"))

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
