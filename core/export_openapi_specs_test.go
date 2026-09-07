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
	GetRoute[string](router, "/api/v1/dummy/test", func(writer http.ResponseWriter, request *http.Request) {},
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

	publicSpec, controlSpec, unifiedSpec, err := ExportOpenAPISpecs()
	require.NoError(t, err)
	require.NotNil(t, publicSpec)
	require.NotNil(t, controlSpec)
	require.NotNil(t, unifiedSpec)

	assert.NotEmpty(t, publicSpec.Paths.Map())
	assert.NotEmpty(t, controlSpec.Paths.Map())
	assert.NotEmpty(t, unifiedSpec.Paths.Map())

	// Public probe route should be present in public and unified
	assert.NotNil(t, publicSpec.Paths.Find("/healthz"))
	assert.NotNil(t, unifiedSpec.Paths.Find("/healthz"))
	assert.Nil(t, controlSpec.Paths.Find("/healthz"))

	// Control plane route should be present in control and unified
	assert.NotNil(t, controlSpec.Paths.Find("/api/v1/_/core/service-accounts"))
	assert.NotNil(t, unifiedSpec.Paths.Find("/api/v1/_/core/service-accounts"))
	assert.Nil(t, publicSpec.Paths.Find("/api/v1/_/core/service-accounts"))

	// Dummy route registered via mock registrar
	assert.NotNil(t, publicSpec.Paths.Find("/api/v1/dummy/test"))
}

func TestCoreExportOpenAPISpecsMergeEdgeCasesUnit(t *testing.T) {
	// 1. Both nil
	mergedNil := MergeOpenAPISpecs(nil, nil)
	require.NotNil(t, mergedNil)
	assert.Equal(t, "3.1.0", mergedNil.OpenAPI)

	// 2. Only public with custom openapi version and empty schemas
	publicOnly := &openapi3.T{
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
	publicOnly.Paths.Set("/test", &openapi3.PathItem{})

	mergedPublic := MergeOpenAPISpecs(publicOnly, nil)
	require.NotNil(t, mergedPublic)
	assert.NotNil(t, mergedPublic.Paths.Find("/test"))
	assert.NotNil(t, mergedPublic.Components.Schemas["TestSchema"])

	// 3. Only control plane with paths and schemas
	controlOnly := &openapi3.T{
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
	controlOnly.Paths.Set("/control", &openapi3.PathItem{})

	mergedControl := MergeOpenAPISpecs(nil, controlOnly)
	require.NotNil(t, mergedControl)
	assert.NotNil(t, mergedControl.Paths.Find("/control"))
	assert.NotNil(t, mergedControl.Components.Schemas["ControlSchema"])
}
