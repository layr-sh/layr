package core

import (
	"reflect"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExportOpenAPISpecs builds and returns the Base, Control Plane, and Unified OpenAPI 3.1 specifications.
func ExportOpenAPISpecs() (*openapi3.T, *openapi3.T, *openapi3.T) {
	log.Debug("exporting OpenAPI 3.1 specifications")
	config := DefaultConfig()

	if config.Security.MasterEncryptionKey == "" {
		config.Security.MasterEncryptionKey, _ = GenerateRandomCryptoEncryptionKeyHex()
	}
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel(nil)
	server := NewServer(kernel)
	kernel.server = server

	serviceFactoriesRWMutex.RLock()
	serviceNames := make([]string, 0, len(serviceFactories))
	for name := range serviceFactories {
		serviceNames = append(serviceNames, name)
	}
	serviceFactoriesRWMutex.RUnlock()
	sort.Strings(serviceNames)
	log.Tracef("registering service routes for OpenAPI export: %v", serviceNames)

	seenRunnerTypes := make(map[reflect.Type]bool)
	for _, serviceName := range serviceNames {
		serviceFactory, ok := GetServiceFactory(serviceName)
		if ok {
			serviceRunner, err := serviceFactory(kernel)
			if err == nil && serviceRunner != nil {
				runnerType := reflect.TypeOf(serviceRunner)
				if seenRunnerTypes[runnerType] {
					continue
				}
				seenRunnerTypes[runnerType] = true
				serviceRunner.RegisterRoutes(server.BaseRouter(), server.ControlPlaneRouter())
			}
		}
	}

	baseOpenAPISpec := server.BaseRouter().OutputOpenAPISpec()
	controlPlaneOpenAPISpec := server.ControlPlaneRouter().OutputOpenAPISpec()
	sanitizeOpenAPISpec(baseOpenAPISpec)
	sanitizeOpenAPISpec(controlPlaneOpenAPISpec)

	unifiedOpenAPISpec := MergeOpenAPISpecs(baseOpenAPISpec, controlPlaneOpenAPISpec)
	sanitizeOpenAPISpec(unifiedOpenAPISpec)
	log.Trace("synthesized base, control plane, and unified OpenAPI specifications")

	return baseOpenAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpec
}

// sanitizeOpenAPISpec normalizes known artifacts produced by Fuego's OpenAPI reflection
// before specifications are validated or consumed by OpenAPI Generator engines.
//
// Why this sanitization is needed:
//
//  1. Empty Schema Names ("") and Broken References ("#/components/schemas/"):
//     When routes define a generic map request/response body like map[string]any (such as data.ExecuteFunctionInput),
//     Fuego's internal type inspection dives into the map's element type (interface{} / any). In Go reflection,
//     interface{} has no type name (t.Name() == ""). Fuego registers this unnamed type under an empty schema
//     key ("") in components.schemas and references it as "#/components/schemas/". OpenAPI Generator strictly
//     validates component schema names against ^[a-zA-Z0-9\.\-_]+$ and crashes on empty references.
//     We strip components.schemas[""] and inline an arbitrary object schema (openapi3.NewObjectSchema()) wherever
//     a broken "#/components/schemas/" ref is encountered.
//
//  2. Standalone "type": "null" on Properties (e.g. json.RawMessage):
//     Struct fields representing arbitrary JSON payloads (such as tasks.Job.TargetPayload) use json.RawMessage
//     ([]byte, kind reflect.Slice). Fuego's built-in schema customizer automatically appends "null" to any slice
//     or map field that lacks a validate:"required" struct tag. Because kin-openapi does not assign a default
//     primitive type to raw byte slices, its type slice is empty ([]). Appending "null" makes the schema strictly
//     type: ["null"]. OpenAPI Generator engines (Python Pydantic models, Go, Rust) fail to generate valid code
//     when a payload property is typed strictly as null. We normalize any standalone "null" types to "object".
func sanitizeOpenAPISpec(openAPISpec *openapi3.T) {
	if openAPISpec == nil {
		return
	}

	// 1. Purge empty component schema name ("") and normalize child schemas
	if openAPISpec.Components != nil && openAPISpec.Components.Schemas != nil {
		delete(openAPISpec.Components.Schemas, "")
		for _, schemaRef := range openAPISpec.Components.Schemas {
			if schemaRef != nil && schemaRef.Value != nil {
				sanitizeSchemaValue(schemaRef.Value)
			}
		}
	}

	// 2. Fix broken "#/components/schemas/" references and standalone nulls in operations
	if openAPISpec.Paths != nil {
		for _, pathItem := range openAPISpec.Paths.Map() {
			if pathItem == nil {
				continue
			}
			for _, operation := range pathItem.Operations() {
				if operation == nil {
					continue
				}

				// Check operation request bodies
				if operation.RequestBody != nil && operation.RequestBody.Value != nil {
					for _, mediaType := range operation.RequestBody.Value.Content {
						if mediaType != nil && mediaType.Schema != nil {
							if mediaType.Schema.Ref == "#/components/schemas/" {
								mediaType.Schema.Ref = ""
								mediaType.Schema.Value = openapi3.NewObjectSchema()
							} else if mediaType.Schema.Value != nil {
								sanitizeSchemaValue(mediaType.Schema.Value)
							}
						}
					}
				}

				// Check operation responses
				if operation.Responses != nil {
					for _, responseRef := range operation.Responses.Map() {
						if responseRef != nil && responseRef.Value != nil {
							for _, mediaType := range responseRef.Value.Content {
								if mediaType != nil && mediaType.Schema != nil {
									if mediaType.Schema.Ref == "#/components/schemas/" {
										mediaType.Schema.Ref = ""
										mediaType.Schema.Value = openapi3.NewObjectSchema()
									} else if mediaType.Schema.Value != nil {
										sanitizeSchemaValue(mediaType.Schema.Value)
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// sanitizeSchemaValue recursively walks an OpenAPI schema AST and normalizes invalid
// standalone "type": "null" declarations to "type": "object".
func sanitizeSchemaValue(schema *openapi3.Schema) {
	if schema == nil {
		return
	}
	if schema.Type != nil {
		typeSlice := schema.Type.Slice()
		if len(typeSlice) == 1 && typeSlice[0] == "null" {
			schema.Type = &openapi3.Types{"object"}
		}
	}
	for _, prop := range schema.Properties {
		if prop != nil && prop.Value != nil {
			sanitizeSchemaValue(prop.Value)
		}
	}
	if schema.Items != nil && schema.Items.Value != nil {
		sanitizeSchemaValue(schema.Items.Value)
	}
}

// MergeOpenAPISpecs combines base and control plane OpenAPI 3.1 specifications into a single unified specification.
func MergeOpenAPISpecs(baseOpenAPISpec, controlPlaneOpenAPISpec *openapi3.T) *openapi3.T {
	unifiedOpenAPISpec := &openapi3.T{
		OpenAPI: "3.1.0",
		Info: &openapi3.Info{
			Title:       "Layr Unified API Engine",
			Version:     "1.0.0",
			Description: "Unified Base Client and Protected Control Plane APIs for Layr Platform",
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: make(openapi3.Schemas),
		},
	}

	if baseOpenAPISpec != nil {
		if baseOpenAPISpec.OpenAPI != "" {
			unifiedOpenAPISpec.OpenAPI = baseOpenAPISpec.OpenAPI
		}
		if baseOpenAPISpec.Paths != nil {
			for _, path := range baseOpenAPISpec.Paths.InMatchingOrder() {
				unifiedOpenAPISpec.Paths.Set(path, baseOpenAPISpec.Paths.Find(path))
			}
		}
		if baseOpenAPISpec.Components != nil && baseOpenAPISpec.Components.Schemas != nil {
			for name, schema := range baseOpenAPISpec.Components.Schemas {
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
