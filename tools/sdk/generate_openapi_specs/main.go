// Package main generates and validates OpenAPI specifications from Go route controllers.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	_ "layr.sh/auth"
	_ "layr.sh/console"
	"layr.sh/core"
	_ "layr.sh/data"
	_ "layr.sh/filestorage"
	_ "layr.sh/function"
	_ "layr.sh/image"
	_ "layr.sh/tasks"
)

const (
	defaultDirPermission  = 0o755
	defaultFilePermission = 0o644
)

func validateSpecOperations(openAPISpec *openapi3.T, specName string) (int, error) {
	if openAPISpec == nil || openAPISpec.Paths == nil {
		return 0, fmt.Errorf("spec %s has no paths defined", specName)
	}

	totalOperations := 0
	for path, pathItem := range openAPISpec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}
			totalOperations++

			// 1. Validate OperationID
			if operation.OperationID == "" {
				return totalOperations, fmt.Errorf("[%s] %s %s missing operationId", specName, method, path)
			}

			// 2. Validate x-sdk-group-name
			rawGroup, hasGroup := operation.Extensions["x-sdk-group-name"]
			if !hasGroup || rawGroup == nil {
				return totalOperations, fmt.Errorf("[%s] %s %s (%s) missing x-sdk-group-name", specName, method, path, operation.OperationID)
			}

			switch group := rawGroup.(type) {
			case []string:
				if len(group) == 0 {
					return totalOperations, fmt.Errorf("[%s] %s %s (%s) has empty x-sdk-group-name", specName, method, path, operation.OperationID)
				}
			case []any:
				if len(group) == 0 {
					return totalOperations, fmt.Errorf("[%s] %s %s (%s) has empty x-sdk-group-name", specName, method, path, operation.OperationID)
				}
			default:
				return totalOperations, fmt.Errorf("[%s] %s %s (%s) x-sdk-group-name has invalid type %T", specName, method, path, operation.OperationID, rawGroup)
			}

			// 3. Validate x-sdk-method-name
			rawMethod, hasMethod := operation.Extensions["x-sdk-method-name"]
			if !hasMethod || rawMethod == nil {
				return totalOperations, fmt.Errorf("[%s] %s %s (%s) missing x-sdk-method-name", specName, method, path, operation.OperationID)
			}

			methodName, ok := rawMethod.(string)
			if !ok || methodName == "" {
				return totalOperations, fmt.Errorf("[%s] %s %s (%s) x-sdk-method-name must be non-empty string, got %v", specName, method, path, operation.OperationID, rawMethod)
			}
		}
	}

	return totalOperations, nil
}

func main() {
	outputDirectoryFlag := flag.String("output-dir", "generated/openapi", "Target directory for exported OpenAPI specifications")
	validateOnlyFlag := flag.Bool("validate-only", false, "Only validate OpenAPI specifications without writing files to disk")
	flag.Parse()

	baseOpenAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpec := core.ExportOpenAPISpecs()

	// Validate base spec
	baseOpsCount, err := validateSpecOperations(baseOpenAPISpec, "base-spec.json")
	if err != nil {
		log.Fatalf("Base spec validation failed: %v", err)
	}

	// Validate control plane spec
	controlPlaneOpsCount, err := validateSpecOperations(controlPlaneOpenAPISpec, "control-plane-spec.json")
	if err != nil {
		log.Fatalf("Control plane spec validation failed: %v", err)
	}

	// Validate unified spec
	unifiedOpsCount, err := validateSpecOperations(unifiedOpenAPISpec, "unified-spec.json")
	if err != nil {
		log.Fatalf("Unified spec validation failed: %v", err)
	}

	if *validateOnlyFlag {
		fmt.Printf("All OpenAPI specifications validated (%d total operations: %d base, %d control plane)\n",
			unifiedOpsCount, baseOpsCount, controlPlaneOpsCount)
		return
	}

	if mkdirErr := os.MkdirAll(*outputDirectoryFlag, defaultDirPermission); mkdirErr != nil {
		log.Fatalf("Failed to create output directory %s: %v", *outputDirectoryFlag, mkdirErr)
	}

	// 1. Export Base Client Spec
	baseSpecPath := filepath.Join(*outputDirectoryFlag, "base-spec.json")
	basePayloadJSON, marshalErr := json.MarshalIndent(baseOpenAPISpec, "", "  ")
	if marshalErr != nil {
		log.Fatalf("Failed to serialize base spec: %v", marshalErr)
	}
	if writeErr := os.WriteFile(baseSpecPath, basePayloadJSON, defaultFilePermission); writeErr != nil {
		log.Fatalf("Failed to write %s: %v", baseSpecPath, writeErr)
	}
	fmt.Printf("Exported base OpenAPI spec: %s (%d paths, %d operations)\n", baseSpecPath, baseOpenAPISpec.Paths.Len(), baseOpsCount)

	// 2. Export Control Plane Spec
	controlPlaneSpecPath := filepath.Join(*outputDirectoryFlag, "control-plane-spec.json")
	controlPlanePayloadJSON, marshalErr := json.MarshalIndent(controlPlaneOpenAPISpec, "", "  ")
	if marshalErr != nil {
		log.Fatalf("Failed to serialize control plane spec: %v", marshalErr)
	}
	if writeErr := os.WriteFile(controlPlaneSpecPath, controlPlanePayloadJSON, defaultFilePermission); writeErr != nil {
		log.Fatalf("Failed to write %s: %v", controlPlaneSpecPath, writeErr)
	}
	fmt.Printf("Exported control plane OpenAPI spec: %s (%d paths, %d operations)\n", controlPlaneSpecPath, controlPlaneOpenAPISpec.Paths.Len(), controlPlaneOpsCount)

	// 3. Export Unified Spec
	unifiedSpecPath := filepath.Join(*outputDirectoryFlag, "unified-spec.json")
	unifiedPayloadJSON, marshalErr := json.MarshalIndent(unifiedOpenAPISpec, "", "  ")
	if marshalErr != nil {
		log.Fatalf("Failed to serialize unified spec: %v", marshalErr)
	}
	if writeErr := os.WriteFile(unifiedSpecPath, unifiedPayloadJSON, defaultFilePermission); writeErr != nil {
		log.Fatalf("Failed to write %s: %v", unifiedSpecPath, writeErr)
	}
	fmt.Printf("Exported unified OpenAPI spec: %s (%d paths, %d operations)\n", unifiedSpecPath, unifiedOpenAPISpec.Paths.Len(), unifiedOpsCount)
}
