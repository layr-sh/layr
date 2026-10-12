package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGenerateOpenapiSpecsValidateSpecOperationsUnit(t *testing.T) {
	t.Run("NilSpec", func(t *testing.T) {
		count, err := validateSpecOperations(nil, "test.json")
		if err == nil || count != 0 {
			t.Fatalf("expected error for nil spec, got count=%d, err=%v", count, err)
		}
	})

	t.Run("NilPaths", func(t *testing.T) {
		openAPISpec := &openapi3.T{Paths: nil}
		count, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil || count != 0 {
			t.Fatalf("expected error for nil paths, got count=%d, err=%v", count, err)
		}
	})

	t.Run("MissingOperationID", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "",
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for missing operationId")
		}
	})

	t.Run("MissingSDKGroupName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions:  map[string]any{},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for missing x-sdk-group-name")
		}
	})

	t.Run("NilSDKGroupName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name": nil,
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for nil x-sdk-group-name")
		}
	})

	t.Run("EmptyStringSliceSDKGroupName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name": []string{},
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for empty []string x-sdk-group-name")
		}
	})

	t.Run("EmptyAnySliceSDKGroupName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name": []any{},
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for empty []any x-sdk-group-name")
		}
	})

	t.Run("InvalidTypeSDKGroupName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name": 12345,
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for invalid type x-sdk-group-name")
		}
	})

	t.Run("MissingSDKMethodName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name": []string{"test"},
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for missing x-sdk-method-name")
		}
	})

	t.Run("NilSDKMethodName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name":  []string{"test"},
					"x-sdk-method-name": nil,
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for nil x-sdk-method-name")
		}
	})

	t.Run("EmptyStringSDKMethodName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name":  []string{"test"},
					"x-sdk-method-name": "",
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for empty string x-sdk-method-name")
		}
	})

	t.Run("InvalidTypeSDKMethodName", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__op",
				Extensions: map[string]any{
					"x-sdk-group-name":  []string{"test"},
					"x-sdk-method-name": 999,
				},
			},
		}
		openAPISpec.Paths.Set("/test", pathItem)

		_, err := validateSpecOperations(openAPISpec, "test.json")
		if err == nil {
			t.Fatal("expected error for invalid type x-sdk-method-name")
		}
	})

	t.Run("ValidOperationsWithStringAndAnySlice", func(t *testing.T) {
		openAPISpec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "test__get",
				Extensions: map[string]any{
					"x-sdk-group-name":  []string{"test", "items"},
					"x-sdk-method-name": "get",
				},
			},
			Post: &openapi3.Operation{
				OperationID: "test__create",
				Extensions: map[string]any{
					"x-sdk-group-name":  []any{"test", "items"},
					"x-sdk-method-name": "create",
				},
			},
		}
		openAPISpec.Paths.Set("/items", pathItem)

		count, err := validateSpecOperations(openAPISpec, "test.json")
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2 operations, got %d", count)
		}
	})
}

func TestGenerateOpenapiSpecsRunUnit(t *testing.T) {
	t.Run("ValidateOnlyFlag", func(t *testing.T) {
		var outputBuffer bytes.Buffer
		err := run([]string{"--validate-only"}, &outputBuffer)
		if err != nil {
			t.Fatalf("run --validate-only failed: %v", err)
		}
		output := outputBuffer.String()
		if !bytes.Contains(outputBuffer.Bytes(), []byte("All OpenAPI specifications validated")) {
			t.Fatalf("expected validation confirmation output, got: %s", output)
		}
	})

	t.Run("OutputDirExportFiles", func(t *testing.T) {
		tempDir := t.TempDir()
		targetOutputDir := filepath.Join(tempDir, "specs")

		var outputBuffer bytes.Buffer
		err := run([]string{"--output-dir=" + targetOutputDir}, &outputBuffer)
		if err != nil {
			t.Fatalf("run --output-dir failed: %v", err)
		}

		expectedFiles := []string{
			"base-spec.json",
			"control-plane-spec.json",
			"unified-spec.json",
		}

		for _, fileName := range expectedFiles {
			filePath := filepath.Join(targetOutputDir, fileName)
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				t.Fatalf("expected file %s to be written: %v", fileName, readErr)
			}
			var parsedJSON map[string]any
			if unmarshalErr := json.Unmarshal(content, &parsedJSON); unmarshalErr != nil {
				t.Fatalf("expected valid JSON in %s: %v", fileName, unmarshalErr)
			}
		}
	})

	t.Run("SpecValidationFailureDuringRun", func(t *testing.T) {
		origFn := exportSpecsFn
		defer func() { exportSpecsFn = origFn }()

		exportSpecsFn = func() (*openapi3.T, *openapi3.T, *openapi3.T) {
			return nil, nil, nil
		}

		var outputBuffer bytes.Buffer
		err := run([]string{"--validate-only"}, &outputBuffer)
		if err == nil {
			t.Fatal("expected error when spec validation fails during run")
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		orig := jsonMarshalIndent
		defer func() { jsonMarshalIndent = orig }()
		jsonMarshalIndent = func(valuePayload any, prefix, indent string) ([]byte, error) {
			return nil, errors.New("mock marshal error")
		}

		var outputBuffer bytes.Buffer
		err := run([]string{"--output-dir=" + t.TempDir()}, &outputBuffer)
		if err == nil {
			t.Fatal("expected error when jsonMarshalIndent fails")
		}
	})

	t.Run("WriteFileError", func(t *testing.T) {
		orig := osWriteFile
		defer func() { osWriteFile = orig }()
		osWriteFile = func(name string, data []byte, fileMode os.FileMode) error {
			return errors.New("mock write error")
		}

		var outputBuffer bytes.Buffer
		err := run([]string{"--output-dir=" + t.TempDir()}, &outputBuffer)
		if err == nil {
			t.Fatal("expected error when osWriteFile fails")
		}
	})

	t.Run("InvalidFlag", func(t *testing.T) {
		var outputBuffer bytes.Buffer
		err := run([]string{"--nonexistent-flag"}, &outputBuffer)
		if err == nil {
			t.Fatal("expected error for nonexistent flag")
		}
	})

	t.Run("MkdirError", func(t *testing.T) {
		tempDir := t.TempDir()
		blockingFile := filepath.Join(tempDir, "blocking_file")
		if writeErr := os.WriteFile(blockingFile, []byte("data"), 0o644); writeErr != nil {
			t.Fatalf("failed to create blocking file: %v", writeErr)
		}

		invalidTargetDir := filepath.Join(blockingFile, "sub_dir")
		var outputBuffer bytes.Buffer
		err := run([]string{"--output-dir=" + invalidTargetDir}, &outputBuffer)
		if err == nil {
			t.Fatal("expected error when MkdirAll fails")
		}
	})
}

func TestGenerateOpenapiSpecsMainEntrypointUnit(t *testing.T) {
	origArgs := os.Args
	origExit := exitFunc
	defer func() {
		os.Args = origArgs
		exitFunc = origExit
	}()

	t.Run("Success", func(t *testing.T) {
		os.Args = []string{"generate_openapi_specs", "--validate-only"}
		main()
	})

	t.Run("Failure", func(t *testing.T) {
		var exitCode int
		exitFunc = func(code int) {
			exitCode = code
		}
		os.Args = []string{"generate_openapi_specs", "--invalid-flag"}
		main()
		if exitCode != 1 {
			t.Fatalf("expected exitCode=1, got %d", exitCode)
		}
	})
}
