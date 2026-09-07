package core

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExportOpenAPISpecs builds and returns the Public, Control Plane, and Unified OpenAPI 3.1 specifications.
func ExportOpenAPISpecs() (*openapi3.T, *openapi3.T, *openapi3.T, error) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Auth.Enabled = true
	config.FileStorage.Enabled = true
	config.Tasks.Enabled = true
	config.Notification.Enabled = true
	config.Analytics.Enabled = true
	config.Image.Enabled = true
	config.Console.Enabled = true

	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	server := NewServer(nil, cryptoKeyManager)

	kernel := &Kernel{
		cryptoKeyManager: cryptoKeyManager,
		server:           server,
	}

	kernel.registerCoreRoutes(server)

	serviceFactoriesMutex.RLock()
	serviceNames := make([]string, 0, len(serviceFactories))
	for name := range serviceFactories {
		serviceNames = append(serviceNames, name)
	}
	serviceFactoriesMutex.RUnlock()
	sort.Strings(serviceNames)

	for _, serviceName := range serviceNames {
		factory, ok := GetServiceFactory(serviceName)
		if ok {
			svc, err := factory(kernel)
			if err == nil {
				svc.RegisterRoutes(server.Router(), server.ControlPlaneRouter())
			}
		}
	}

	publicSpec := server.Router().OutputOpenAPISpec()
	controlSpec := server.ControlPlaneRouter().OutputOpenAPISpec()
	unifiedSpec := MergeOpenAPISpecs(publicSpec, controlSpec)

	return publicSpec, controlSpec, unifiedSpec, nil
}

// MergeOpenAPISpecs combines public and control plane OpenAPI 3.1 specifications into a single unified specification.
func MergeOpenAPISpecs(publicSpec, controlSpec *openapi3.T) *openapi3.T {
	unifiedSpec := &openapi3.T{
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

	if publicSpec != nil {
		if publicSpec.OpenAPI != "" {
			unifiedSpec.OpenAPI = publicSpec.OpenAPI
		}
		if publicSpec.Paths != nil {
			for _, path := range publicSpec.Paths.InMatchingOrder() {
				unifiedSpec.Paths.Set(path, publicSpec.Paths.Find(path))
			}
		}
		if publicSpec.Components != nil && publicSpec.Components.Schemas != nil {
			for name, schema := range publicSpec.Components.Schemas {
				unifiedSpec.Components.Schemas[name] = schema
			}
		}
	}

	if controlSpec != nil {
		if controlSpec.Paths != nil {
			for _, path := range controlSpec.Paths.InMatchingOrder() {
				unifiedSpec.Paths.Set(path, controlSpec.Paths.Find(path))
			}
		}
		if controlSpec.Components != nil && controlSpec.Components.Schemas != nil {
			for name, schema := range controlSpec.Components.Schemas {
				unifiedSpec.Components.Schemas[name] = schema
			}
		}
	}

	return unifiedSpec
}
