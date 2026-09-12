package core

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExportOpenAPISpecs builds and returns the Public, Control Plane, and Unified OpenAPI 3.1 specifications.
func ExportOpenAPISpecs() (*openapi3.T, *openapi3.T, *openapi3.T, error) {
	log.Debug("exporting OpenAPI 3.1 specifications")
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Auth.Enabled = true
	config.FileStorage.Enabled = true
	config.Tasks.Enabled = true
	config.Notification.Enabled = true
	config.Analytics.Enabled = true
	config.Image.Enabled = true
	config.Console.Enabled = true

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	server := NewServer(nil, cryptoKeyManager)

	kernel := &Kernel{
		cryptoKeyManager: cryptoKeyManager,
		server:           server,
	}

	kernel.registerCoreRoutes(server)

	serviceFactoriesRWMutex.RLock()
	serviceNames := make([]string, 0, len(serviceFactories))
	for name := range serviceFactories {
		serviceNames = append(serviceNames, name)
	}
	serviceFactoriesRWMutex.RUnlock()
	sort.Strings(serviceNames)
	log.Tracef("registering service routes for OpenAPI export: %v", serviceNames)

	for _, serviceName := range serviceNames {
		serviceFactory, ok := GetServiceFactory(serviceName)
		if ok {
			serviceRunner, err := serviceFactory(kernel)
			if err == nil {
				serviceRunner.RegisterRoutes(server.BaseRouter(), server.ControlPlaneRouter())
			}
		}
	}

	openAPISpec := server.BaseRouter().OutputOpenAPISpec()
	controlPlaneOpenAPISpec := server.ControlPlaneRouter().OutputOpenAPISpec()
	unifiedOpenAPISpec := MergeOpenAPISpecs(openAPISpec, controlPlaneOpenAPISpec)
	log.Trace("synthesized public, control plane, and unified OpenAPI specifications")

	return openAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpec, nil
}

// MergeOpenAPISpecs combines public and control plane OpenAPI 3.1 specifications into a single unified specification.
func MergeOpenAPISpecs(openAPISpec, controlPlaneOpenAPISpec *openapi3.T) *openapi3.T {
	unifiedOpenAPISpec := &openapi3.T{
		OpenAPI: "3.1.0",
		Info: &openapi3.Info{
			Title:       "Layr Unified API Engine",
			Version:     "1.0.0",
			Description: "Unified Public Client and Protected Control Plane APIs for Layr Platform",
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: make(openapi3.Schemas),
		},
	}

	if openAPISpec != nil {
		if openAPISpec.OpenAPI != "" {
			unifiedOpenAPISpec.OpenAPI = openAPISpec.OpenAPI
		}
		if openAPISpec.Paths != nil {
			for _, path := range openAPISpec.Paths.InMatchingOrder() {
				unifiedOpenAPISpec.Paths.Set(path, openAPISpec.Paths.Find(path))
			}
		}
		if openAPISpec.Components != nil && openAPISpec.Components.Schemas != nil {
			for name, schema := range openAPISpec.Components.Schemas {
				unifiedOpenAPISpec.Components.Schemas[name] = schema
			}
		}
	}

	if controlPlaneOpenAPISpec != nil {
		if controlPlaneOpenAPISpec.Paths != nil {
			for _, path := range controlPlaneOpenAPISpec.Paths.InMatchingOrder() {
				unifiedOpenAPISpec.Paths.Set(path, controlPlaneOpenAPISpec.Paths.Find(path))
			}
		}
		if controlPlaneOpenAPISpec.Components != nil && controlPlaneOpenAPISpec.Components.Schemas != nil {
			for name, schema := range controlPlaneOpenAPISpec.Components.Schemas {
				unifiedOpenAPISpec.Components.Schemas[name] = schema
			}
		}
	}

	return unifiedOpenAPISpec
}
