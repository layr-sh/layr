package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGenerateDevReferencesServiceIdentifiersAndDisplayNamesUnit(t *testing.T) {
	services := []struct {
		service          string
		expectedDisplay  string
		expectedVariable string
		expectedHook     string
	}{
		{"auth", "Auth", "auth", "useAuth"},
		{"data", "Data", "data", "useData"},
		{"file_storage", "File Storage", "fileStorage", "useFileStorage"},
		{"filestorage", "File Storage", "fileStorage", "useFileStorage"},
		{"image", "Image", "image", "useImage"},
		{"function", "Function", "function", "useFunction"},
		{"tasks", "Tasks", "tasks", "useTasks"},
		{"core", "Core", "core", "useCore"},
		{"console", "Console", "console", "useConsole"},
		{"billing", "Billing", "billing", "useBilling"},
	}

	for _, testCase := range services {
		t.Run(testCase.service, func(t *testing.T) {
			display := getServiceDisplayName(testCase.service)
			if display != testCase.expectedDisplay {
				t.Errorf("getServiceDisplayName(%q) = %q, want %q", testCase.service, display, testCase.expectedDisplay)
			}

			variable := getServiceVariableIdentifier(testCase.service)
			if variable != testCase.expectedVariable {
				t.Errorf("getServiceVariableIdentifier(%q) = %q, want %q", testCase.service, variable, testCase.expectedVariable)
			}

			hook := getServiceHookIdentifier(testCase.service)
			if hook != testCase.expectedHook {
				t.Errorf("getServiceHookIdentifier(%q) = %q, want %q", testCase.service, hook, testCase.expectedHook)
			}
		})
	}
}

func TestGenerateDevReferencesReplaceBetweenMarkersUnit(t *testing.T) {
	const (
		startMarker = "<!-- START -->"
		endMarker   = "<!-- END -->"
	)

	t.Run("ValidReplacement", func(t *testing.T) {
		initial := "Prefix " + startMarker + " Old Content " + endMarker + " Suffix"
		result, err := replaceBetweenMarkers(initial, startMarker, endMarker, " New Content ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "Prefix " + startMarker + " New Content " + endMarker + " Suffix"
		if result != expected {
			t.Fatalf("got %q, want %q", result, expected)
		}
	})

	t.Run("MissingStartMarker", func(t *testing.T) {
		initial := "Prefix " + endMarker + " Suffix"
		_, err := replaceBetweenMarkers(initial, startMarker, endMarker, " New ")
		if err == nil || !strings.Contains(err.Error(), "marker \"<!-- START -->\" not found") {
			t.Fatalf("expected missing start marker error, got: %v", err)
		}
	})

	t.Run("MissingEndMarker", func(t *testing.T) {
		initial := "Prefix " + startMarker + " Suffix"
		_, err := replaceBetweenMarkers(initial, startMarker, endMarker, " New ")
		if err == nil || !strings.Contains(err.Error(), "marker \"<!-- END -->\" not found") {
			t.Fatalf("expected missing end marker error, got: %v", err)
		}
	})

	t.Run("EndMarkerBeforeStartMarker", func(t *testing.T) {
		initial := "Prefix " + endMarker + " Middle " + startMarker + " Suffix"
		_, err := replaceBetweenMarkers(initial, startMarker, endMarker, " New ")
		if err == nil || !strings.Contains(err.Error(), "appears before") {
			t.Fatalf("expected marker order error, got: %v", err)
		}
	})
}

func TestGenerateDevReferencesGetOperationsUnit(t *testing.T) {
	t.Run("NilSpec", func(t *testing.T) {
		operations := getOperations(nil, "base")
		if operations != nil {
			t.Fatalf("expected nil, got %v", operations)
		}
	})

	t.Run("NilPaths", func(t *testing.T) {
		openAPISpec := &openapi3.T{Paths: nil}
		operations := getOperations(openAPISpec, "base")
		if operations != nil {
			t.Fatalf("expected nil, got %v", operations)
		}
	})

	t.Run("EmptyPaths", func(t *testing.T) {
		openAPISpec := &openapi3.T{Paths: openapi3.NewPaths()}
		operations := getOperations(openAPISpec, "base")
		if len(operations) != 0 {
			t.Fatalf("expected 0 operations, got %d", len(operations))
		}
	})

	t.Run("ValidOperationsWithVariousExtensions", func(t *testing.T) {
		openAPISpec := &openapi3.T{Paths: openapi3.NewPaths()}
		pathItem := &openapi3.PathItem{
			Get: &openapi3.Operation{
				OperationID: "data__records__list",
				Summary:     "List records",
				Extensions: map[string]any{
					"x-sdk-group-name":  []string{"data", "records"},
					"x-sdk-method-name": "list",
				},
			},
			Post: &openapi3.Operation{
				OperationID: "auth__login",
				Summary:     "Login user",
				Extensions: map[string]any{
					"x-sdk-group-name":  []any{"auth", 123, "users"},
					"x-sdk-method-name": "login",
				},
			},
			Delete: &openapi3.Operation{
				OperationID: "core__ping",
				Summary:     "Ping core",
				Extensions:  map[string]any{},
			},
		}
		openAPISpec.Paths.Set("/api/test", pathItem)

		operations := getOperations(openAPISpec, "base")
		if len(operations) != 3 {
			t.Fatalf("expected 3 operations, got %d", len(operations))
		}

		opMap := make(map[string]OperationRecord)
		for _, operation := range operations {
			opMap[operation.OperationID] = operation
		}

		recordOperationRecord := opMap["data__records__list"]
		if recordOperationRecord.Service != "data" || recordOperationRecord.SubGroup != "records" || recordOperationRecord.SDKMethod != "list" {
			t.Errorf("unexpected record op: %+v", recordOperationRecord)
		}

		authOperationRecord := opMap["auth__login"]
		if authOperationRecord.Service != "auth" || authOperationRecord.SubGroup != "users" || authOperationRecord.SDKMethod != "login" {
			t.Errorf("unexpected auth op: %+v", authOperationRecord)
		}

		coreOperationRecord := opMap["core__ping"]
		if coreOperationRecord.Service != "core" || coreOperationRecord.SubGroup != "" || coreOperationRecord.SDKMethod != "" {
			t.Errorf("unexpected core op: %+v", coreOperationRecord)
		}
	})
}

func TestGenerateDevReferencesRenderPlaneOperationsUnit(t *testing.T) {
	operations := []OperationRecord{
		{
			Plane:       "base",
			Service:     "auth",
			SubGroup:    "",
			HTTPMethod:  "POST",
			Path:        "/v1/auth/login",
			OperationID: "auth__login",
			SDKGroups:   []string{"auth"},
			SDKMethod:   "login",
			Summary:     "Login endpoint",
		},
		{
			Plane:       "base",
			Service:     "data",
			SubGroup:    "records",
			HTTPMethod:  "GET",
			Path:        "/v1/data/records",
			OperationID: "data__records__list",
			SDKGroups:   []string{"data", "records"},
			SDKMethod:   "list",
			Summary:     "List records",
		},
	}

	t.Run("BasePlaneRendering", func(t *testing.T) {
		var outputBuffer bytes.Buffer
		renderPlaneOperations(&outputBuffer, operations, false)
		rendered := outputBuffer.String()

		if !strings.Contains(rendered, "#### `auth`") {
			t.Error("expected auth service section")
		}
		if !strings.Contains(rendered, "auth.login()") {
			t.Error("expected auth.login() call signature")
		}
		if !strings.Contains(rendered, "Subgroup `data.records.*`:") {
			t.Error("expected data subgroup")
		}
		if !strings.Contains(rendered, "data.records.list()") {
			t.Error("expected data.records.list() call signature")
		}
		if !strings.Contains(rendered, "#### `function`") {
			t.Error("expected function note on base plane")
		}
	})

	t.Run("ControlPlaneRendering", func(t *testing.T) {
		controlPlaneOperationsRecord := []OperationRecord{
			{
				Plane:       "control_plane",
				Service:     "auth",
				SubGroup:    "",
				HTTPMethod:  "POST",
				Path:        "/v1/auth/users",
				OperationID: "auth__users__create",
				SDKGroups:   []string{"auth"},
				SDKMethod:   "create",
				Summary:     "Create user",
			},
			{
				Plane:       "control_plane",
				Service:     "data",
				SubGroup:    "tables",
				HTTPMethod:  "POST",
				Path:        "/v1/data/tables",
				OperationID: "data__tables__create",
				SDKGroups:   []string{"data", "tables"},
				SDKMethod:   "create",
				Summary:     "Create table",
			},
		}

		var outputBuffer bytes.Buffer
		renderPlaneOperations(&outputBuffer, controlPlaneOperationsRecord, true)
		rendered := outputBuffer.String()

		if !strings.Contains(rendered, "auth.cp.create()") {
			t.Error("expected auth.cp.create() call signature")
		}
		if !strings.Contains(rendered, "data.cp.tables.create()") {
			t.Error("expected data.cp.tables.create() call signature")
		}
		if strings.Contains(rendered, "The Function service has no base plane endpoints") {
			t.Error("should not contain base plane notes on control plane")
		}
	})
}

func TestGenerateDevReferencesFindRepoRootUnit(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		root, err := findRepoRoot()
		if err != nil {
			t.Fatalf("findRepoRoot failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			t.Fatalf("expected go.mod in repo root %s: %v", root, err)
		}
	})

	t.Run("GetwdError", func(t *testing.T) {
		orig := getwdFn
		defer func() { getwdFn = orig }()
		getwdFn = func() (string, error) {
			return "", errors.New("mock getwd failure")
		}

		_, err := findRepoRoot()
		if err == nil || !strings.Contains(err.Error(), "mock getwd failure") {
			t.Fatalf("expected getwd error, got: %v", err)
		}
	})

	t.Run("NotFoundTraversingToRoot", func(t *testing.T) {
		orig := getwdFn
		defer func() { getwdFn = orig }()
		// Point to root directory "/" where go.mod does not exist
		getwdFn = func() (string, error) {
			return string(filepath.Separator), nil
		}

		_, err := findRepoRoot()
		if err == nil || !strings.Contains(err.Error(), "could not find repo root") {
			t.Fatalf("expected could not find repo root error, got: %v", err)
		}
	})
}

func TestGenerateDevReferencesGenerateMethodsMarkdownUnit(t *testing.T) {
	baseMarkdown, controlPlaneMarkdown := generateMethodsMarkdown()
	if len(baseMarkdown) == 0 {
		t.Fatal("expected non-empty baseMarkdown")
	}
	if len(controlPlaneMarkdown) == 0 {
		t.Fatal("expected non-empty controlPlaneMarkdown")
	}
}

func TestGenerateDevReferencesRunUnit(t *testing.T) {
	t.Run("PrintFlag", func(t *testing.T) {
		var stdoutBuffer bytes.Buffer
		err := run([]string{"--print"}, &stdoutBuffer)
		if err != nil {
			t.Fatalf("unexpected error with --print: %v", err)
		}
		output := stdoutBuffer.String()
		if !strings.Contains(output, "=== BASE PLANE METHODS ===") {
			t.Error("expected base plane header in stdout")
		}
		if !strings.Contains(output, "=== CONTROL PLANE (.cp) METHODS ===") {
			t.Error("expected control plane header in stdout")
		}
	})

	t.Run("InvalidFlag", func(t *testing.T) {
		var stdoutBuffer bytes.Buffer
		err := run([]string{"--invalid-flag"}, &stdoutBuffer)
		if err == nil {
			t.Fatal("expected error with invalid flag")
		}
	})

	t.Run("DefaultRepoRootExecution", func(t *testing.T) {
		var stdoutBuffer bytes.Buffer
		err := run([]string{"--check"}, &stdoutBuffer)
		if err != nil {
			t.Fatalf("run --check failed on repository root: %v", err)
		}
	})

	t.Run("DefaultRepoRootError", func(t *testing.T) {
		orig := getwdFn
		defer func() { getwdFn = orig }()
		getwdFn = func() (string, error) {
			return "", errors.New("mock getwd error")
		}

		var stdoutBuffer bytes.Buffer
		err := run([]string{"--check"}, &stdoutBuffer)
		if err == nil {
			t.Fatal("expected error when findRepoRoot fails")
		}
	})

	t.Run("UpdateAndCheckTempReadme", func(t *testing.T) {
		tempDir := t.TempDir()
		readmePath := filepath.Join(tempDir, "README.md")

		initialReadme := "# Client SDK\n\n" +
			baseStartMarker + "\nold base\n" + baseEndMarker + "\n\n" +
			controlPlaneStartMarker + "\nold cp\n" + controlPlaneEndMarker + "\n"

		if err := os.WriteFile(readmePath, []byte(initialReadme), 0o644); err != nil {
			t.Fatalf("failed to create temp README: %v", err)
		}

		// 1. First run --check on stale readme -> should fail
		var stdoutBuffer bytes.Buffer
		err := run([]string{"--check"}, &stdoutBuffer, readmePath)
		if err == nil || !strings.Contains(err.Error(), "out of date") {
			t.Fatalf("expected drift error on outdated README, got: %v", err)
		}

		// 2. Run update on readme
		stdoutBuffer.Reset()
		err = run([]string{}, &stdoutBuffer, readmePath)
		if err != nil {
			t.Fatalf("failed to update README: %v", err)
		}
		if !strings.Contains(stdoutBuffer.String(), "Updated") {
			t.Errorf("expected Updated message, got: %s", stdoutBuffer.String())
		}

		// 3. Now run --check on freshly updated readme -> should succeed
		stdoutBuffer.Reset()
		err = run([]string{"--check"}, &stdoutBuffer, readmePath)
		if err != nil {
			t.Fatalf("expected --check to pass on synced README: %v", err)
		}
		if !strings.Contains(stdoutBuffer.String(), "is up to date") {
			t.Errorf("expected up to date message, got: %s", stdoutBuffer.String())
		}
	})

	t.Run("WriteFileError", func(t *testing.T) {
		tempDir := t.TempDir()
		readmePath := filepath.Join(tempDir, "README.md")
		initialReadme := "# Client SDK\n\n" +
			baseStartMarker + "\n" + baseEndMarker + "\n\n" +
			controlPlaneStartMarker + "\n" + controlPlaneEndMarker + "\n"
		if err := os.WriteFile(readmePath, []byte(initialReadme), 0o644); err != nil {
			t.Fatalf("failed to create README: %v", err)
		}

		orig := osWriteFile
		defer func() { osWriteFile = orig }()
		osWriteFile = func(name string, data []byte, fileMode os.FileMode) error {
			return errors.New("mock write error")
		}

		var stdoutBuffer bytes.Buffer
		err := run([]string{}, &stdoutBuffer, readmePath)
		if err == nil || !strings.Contains(err.Error(), "mock write error") {
			t.Fatalf("expected write error, got: %v", err)
		}
	})

	t.Run("MissingControlPlaneMarkerError", func(t *testing.T) {
		tempDir := t.TempDir()
		readmePath := filepath.Join(tempDir, "README.md")
		content := "# Header\n\n" + baseStartMarker + "\n" + baseEndMarker + "\n"
		if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create README: %v", err)
		}

		var stdoutBuffer bytes.Buffer
		err := run([]string{}, &stdoutBuffer, readmePath)
		if err == nil || !strings.Contains(err.Error(), "injecting control plane methods") {
			t.Fatalf("expected control plane injection error, got: %v", err)
		}
	})

	t.Run("MissingReadmeFileError", func(t *testing.T) {
		var stdoutBuffer bytes.Buffer
		err := run([]string{}, &stdoutBuffer, "/nonexistent/path/README.md")
		if err == nil {
			t.Fatal("expected error reading nonexistent README")
		}
	})

	t.Run("MissingMarkerError", func(t *testing.T) {
		tempDir := t.TempDir()
		readmePath := filepath.Join(tempDir, "README.md")
		if err := os.WriteFile(readmePath, []byte("# Empty"), 0o644); err != nil {
			t.Fatalf("failed to create temp README: %v", err)
		}

		var stdoutBuffer bytes.Buffer
		err := run([]string{}, &stdoutBuffer, readmePath)
		if err == nil || !strings.Contains(err.Error(), "marker") {
			t.Fatalf("expected marker error, got: %v", err)
		}
	})
}

func TestGenerateDevReferencesMainEntrypointUnit(t *testing.T) {
	origArgs := os.Args
	origExit := exitFunc
	defer func() {
		os.Args = origArgs
		exitFunc = origExit
	}()

	t.Run("Success", func(t *testing.T) {
		os.Args = []string{"generate_dev_references", "--print"}
		main()
	})

	t.Run("Failure", func(t *testing.T) {
		var exitCode int
		exitFunc = func(code int) {
			exitCode = code
		}
		os.Args = []string{"generate_dev_references", "--invalid-flag"}
		main()
		if exitCode != 1 {
			t.Fatalf("expected exitCode=1, got %d", exitCode)
		}
	})
}
