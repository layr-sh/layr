// Package main generates and validates OpenAPI specifications from Go route controllers.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

var (
	exportSpecsFn     = core.ExportOpenAPISpecs
	jsonMarshalIndent = json.MarshalIndent
	osWriteFile       = os.WriteFile
	exitFunc          = os.Exit
)

func run(args []string, stdoutWriter io.Writer) error {
	flagSet := flag.NewFlagSet("generate_openapi_specs", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	outputDirectoryFlag := flagSet.String("output-dir", "generated/openapi", "Target directory for exported OpenAPI specifications")
	validateOnlyFlag := flagSet.Bool("validate-only", false, "Only validate OpenAPI specifications without writing files to disk")
	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	baseOpenAPISpec, controlPlaneOpenAPISpec, unifiedOpenAPISpec := exportSpecsFn()

	specs := []struct {
		name        string
		fileName    string
		openAPISpec *openapi3.T
	}{
		{"base", "base-spec.json", baseOpenAPISpec},
		{"control plane", "control-plane-spec.json", controlPlaneOpenAPISpec},
		{"unified", "unified-spec.json", unifiedOpenAPISpec},
	}

	opsCounts := make([]int, len(specs))
	for index, targetSpec := range specs {
		count, err := validateSpecOperations(targetSpec.openAPISpec, targetSpec.fileName)
		if err != nil {
			return fmt.Errorf("%s spec validation failed: %w", targetSpec.name, err)
		}
		opsCounts[index] = count
	}

	if *validateOnlyFlag {
		_, _ = fmt.Fprintf(stdoutWriter, "All OpenAPI specifications validated (%d total operations: %d base, %d control plane)\n",
			opsCounts[2], opsCounts[0], opsCounts[1])
		return nil
	}

	if mkdirErr := os.MkdirAll(*outputDirectoryFlag, defaultDirPermission); mkdirErr != nil {
		return fmt.Errorf("failed to create output directory %s: %w", *outputDirectoryFlag, mkdirErr)
	}

	for index, targetSpec := range specs {
		specPath := filepath.Join(*outputDirectoryFlag, targetSpec.fileName)
		payloadJSON, marshalErr := jsonMarshalIndent(targetSpec.openAPISpec, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to serialize %s spec: %w", targetSpec.name, marshalErr)
		}
		if writeErr := osWriteFile(specPath, payloadJSON, defaultFilePermission); writeErr != nil {
			return fmt.Errorf("failed to write %s: %w", specPath, writeErr)
		}
		_, _ = fmt.Fprintf(stdoutWriter, "Exported %s OpenAPI spec: %s (%d paths, %d operations)\n", targetSpec.name, specPath, targetSpec.openAPISpec.Paths.Len(), opsCounts[index])
	}

	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		exitFunc(1)
	}
}
