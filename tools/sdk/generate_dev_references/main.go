// Package main generates client SDK documentation and method checklists from Go route controllers.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

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
	defaultFilePermission = 0o644

	baseStartMarker         = "<!-- BEGIN_GENERATED_BASE_METHODS -->"
	baseEndMarker           = "<!-- END_GENERATED_BASE_METHODS -->"
	controlPlaneStartMarker = "<!-- BEGIN_GENERATED_CONTROL_PLANE_METHODS -->"
	controlPlaneEndMarker   = "<!-- END_GENERATED_CONTROL_PLANE_METHODS -->"
)

// OperationRecord encapsulates an OpenAPI route operation mapped to its SDK invocation hierarchy.
type OperationRecord struct {
	Plane       string   // "base" or "control_plane"
	Service     string   // e.g. "auth", "data", "file_storage", "image", "function", "tasks", "core", "console"
	SubGroup    string   // groups after service name, e.g. "records", "users.sessions"
	HTTPMethod  string   // "GET", "POST", etc.
	Path        string   // "/v1/data/..."
	OperationID string   // "data__records__list"
	SDKGroups   []string // ["data", "records"]
	SDKMethod   string   // "list"
	Summary     string   // "List records in a table"
}

func getOperations(openAPISpec *openapi3.T, plane string) []OperationRecord {
	if openAPISpec == nil || openAPISpec.Paths == nil {
		return nil
	}
	var operations []OperationRecord
	for path, pathItem := range openAPISpec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}
			var sdkGroups []string
			if raw, hasGroups := operation.Extensions["x-sdk-group-name"]; hasGroups {
				switch typedGroups := raw.(type) {
				case []string:
					sdkGroups = typedGroups
				case []any:
					for _, item := range typedGroups {
						if stringItem, isString := item.(string); isString {
							sdkGroups = append(sdkGroups, stringItem)
						}
					}
				}
			}
			var sdkMethod string
			if raw, hasMethod := operation.Extensions["x-sdk-method-name"]; hasMethod {
				if stringMethod, isString := raw.(string); isString {
					sdkMethod = stringMethod
				}
			}

			service := "core"
			if len(sdkGroups) > 0 {
				service = sdkGroups[0]
			}
			subGroups := ""
			if len(sdkGroups) > 1 {
				subGroups = strings.Join(sdkGroups[1:], ".")
			}

			operations = append(operations, OperationRecord{
				Plane:       plane,
				Service:     service,
				SubGroup:    subGroups,
				HTTPMethod:  strings.ToUpper(method),
				Path:        path,
				OperationID: operation.OperationID,
				SDKGroups:   sdkGroups,
				SDKMethod:   sdkMethod,
				Summary:     operation.Summary,
			})
		}
	}
	return operations
}

func generateMethodsMarkdown() (string, string) {
	baseOpenAPISpec, controlPlaneOpenAPISpec, _ := core.ExportOpenAPISpecs()
	baseOperations := getOperations(baseOpenAPISpec, "base")
	controlPlaneOperations := getOperations(controlPlaneOpenAPISpec, "control_plane")

	sortOperations := func(records []OperationRecord) {
		sort.Slice(records, func(leftIndex, rightIndex int) bool {
			if records[leftIndex].Service != records[rightIndex].Service {
				return records[leftIndex].Service < records[rightIndex].Service
			}
			groupLeft := strings.Join(records[leftIndex].SDKGroups, ".")
			groupRight := strings.Join(records[rightIndex].SDKGroups, ".")
			if groupLeft != groupRight {
				return groupLeft < groupRight
			}
			return records[leftIndex].SDKMethod < records[rightIndex].SDKMethod
		})
	}
	sortOperations(baseOperations)
	sortOperations(controlPlaneOperations)

	var baseMarkdownBuffer bytes.Buffer
	renderPlaneOperations(&baseMarkdownBuffer, baseOperations, false)

	var controlPlaneMarkdownBuffer bytes.Buffer
	renderPlaneOperations(&controlPlaneMarkdownBuffer, controlPlaneOperations, true)

	return baseMarkdownBuffer.String(), controlPlaneMarkdownBuffer.String()
}

func renderPlaneOperations(markdownBuffer *bytes.Buffer, operations []OperationRecord, isControlPlane bool) {
	servicesMap := make(map[string][]OperationRecord)
	var serviceList []string
	for _, operation := range operations {
		if _, exists := servicesMap[operation.Service]; !exists {
			serviceList = append(serviceList, operation.Service)
		}
		servicesMap[operation.Service] = append(servicesMap[operation.Service], operation)
	}
	sort.Strings(serviceList)

	for _, service := range serviceList {
		serviceOperations := servicesMap[service]
		displayName := getServiceDisplayName(service)
		clientVariableName := getServiceVariableIdentifier(service)
		hookName := getServiceHookIdentifier(service)

		if isControlPlane {
			fmt.Fprintf(markdownBuffer, "#### `%s.cp` (%s Control Plane — via `withServiceAccount(%s(client), ...)`)\n\n",
				clientVariableName, displayName, hookName)
		} else {
			fmt.Fprintf(markdownBuffer, "#### `%s` (%s Service — via `%s(client)`)\n\n",
				clientVariableName, displayName, hookName)
		}

		// Group by sub-group
		subGroupMap := make(map[string][]OperationRecord)
		var subGroups []string
		for _, operation := range serviceOperations {
			subGroupName := operation.SubGroup
			if subGroupName == "" {
				subGroupName = "(root)"
			}
			if _, exists := subGroupMap[subGroupName]; !exists {
				subGroups = append(subGroups, subGroupName)
			}
			subGroupMap[subGroupName] = append(subGroupMap[subGroupName], operation)
		}
		sort.Strings(subGroups)

		for _, subGroupName := range subGroups {
			groupOperations := subGroupMap[subGroupName]
			if subGroupName != "(root)" {
				if isControlPlane {
					fmt.Fprintf(markdownBuffer, "- Subgroup `%s.cp.%s.*`:\n", clientVariableName, subGroupName)
				} else {
					fmt.Fprintf(markdownBuffer, "- Subgroup `%s.%s.*`:\n", clientVariableName, subGroupName)
				}
			} else {
				if isControlPlane {
					fmt.Fprintf(markdownBuffer, "- Root methods (`%s.cp.*`):\n", clientVariableName)
				} else {
					fmt.Fprintf(markdownBuffer, "- Root methods (`%s.*`):\n", clientVariableName)
				}
			}
			for _, operation := range groupOperations {
				callSignature := ""
				if isControlPlane {
					if operation.SubGroup != "" {
						callSignature = fmt.Sprintf("%s.cp.%s.%s()", clientVariableName, operation.SubGroup, operation.SDKMethod)
					} else {
						callSignature = fmt.Sprintf("%s.cp.%s()", clientVariableName, operation.SDKMethod)
					}
				} else {
					if operation.SubGroup != "" {
						callSignature = fmt.Sprintf("%s.%s.%s()", clientVariableName, operation.SubGroup, operation.SDKMethod)
					} else {
						callSignature = fmt.Sprintf("%s.%s()", clientVariableName, operation.SDKMethod)
					}
				}
				fmt.Fprintf(markdownBuffer, "  - [ ] `%s`: %s (`%s %s` | `%s`)\n",
					callSignature, operation.Summary, operation.HTTPMethod, operation.Path, operation.OperationID)
			}
			fmt.Fprintln(markdownBuffer)
		}
	}

	if !isControlPlane {
		fmt.Fprintln(markdownBuffer, "#### `tasks` (Tasks Service — via `useTasks(client)`)")
		fmt.Fprintln(markdownBuffer, "The Tasks service has no base plane endpoints. All tasks operations exist on the control plane via `tasks.cp.*` upon service account elevation (`withServiceAccount(useTasks(client), ...)`).")

		fmt.Fprintln(markdownBuffer, "#### `console` (Console Service — via `useConsole(client)`)")
		fmt.Fprintln(markdownBuffer, "The Console service has no base plane endpoints. All console operations exist on the control plane via `console.cp.*` upon service account elevation (`withServiceAccount(useConsole(client), ...)`).")
	}
}

func getServiceDisplayName(service string) string {
	switch service {
	case "auth":
		return "Auth"
	case "data":
		return "Data"
	case "file_storage", "filestorage":
		return "File Storage"
	case "image":
		return "Image"
	case "function":
		return "Function"
	case "tasks":
		return "Tasks"
	case "core":
		return "Core"
	case "console":
		return "Console"
	default:
		return strings.ToUpper(service[:1]) + service[1:]
	}
}

func getServiceVariableIdentifier(service string) string {
	switch service {
	case "auth":
		return "auth"
	case "data":
		return "data"
	case "file_storage", "filestorage":
		return "fileStorage"
	case "image":
		return "image"
	case "function":
		return "function"
	case "tasks":
		return "tasks"
	case "core":
		return "core"
	case "console":
		return "console"
	default:
		return service
	}
}

func getServiceHookIdentifier(service string) string {
	switch service {
	case "auth":
		return "useAuth"
	case "data":
		return "useData"
	case "file_storage", "filestorage":
		return "useFileStorage"
	case "image":
		return "useImage"
	case "function":
		return "useFunction"
	case "tasks":
		return "useTasks"
	case "core":
		return "useCore"
	case "console":
		return "useConsole"
	default:
		return "use" + getServiceDisplayName(service)
	}
}

func main() {
	checkFlag := flag.Bool("check", false, "Verify that client-sdk/README.md is up to date with router methods")
	printFlag := flag.Bool("print", false, "Print generated method markdown to stdout instead of updating file")
	flag.Parse()

	baseMarkdown, controlPlaneMarkdown := generateMethodsMarkdown()

	if *printFlag {
		fmt.Println("=== BASE PLANE METHODS ===")
		fmt.Println(baseMarkdown)
		fmt.Println("=== CONTROL PLANE (.cp) METHODS ===")
		fmt.Println(controlPlaneMarkdown)
		return
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	readmePath := filepath.Join(repoRoot, "client-sdk", "README.md")
	contentBytes, err := os.ReadFile(readmePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", readmePath, err)
		os.Exit(1)
	}
	content := string(contentBytes)

	newContent := content
	newContent, err = replaceBetweenMarkers(newContent, baseStartMarker, baseEndMarker, "\n\n"+baseMarkdown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error injecting base methods: %v\n", err)
		os.Exit(1)
	}

	newContent, err = replaceBetweenMarkers(newContent, controlPlaneStartMarker, controlPlaneEndMarker, "\n\n"+controlPlaneMarkdown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error injecting control plane methods: %v\n", err)
		os.Exit(1)
	}

	if *checkFlag {
		if newContent != content {
			fmt.Fprintln(os.Stderr, "client-sdk/README.md is out of date with Go router routes. Run ./scripts/sdk/generate-dev-references to synchronize.")
			os.Exit(1)
		}
		fmt.Println("client-sdk/README.md is up to date with all router methods.")
		return
	}

	if err := os.WriteFile(readmePath, []byte(newContent), defaultFilePermission); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", readmePath, err)
		os.Exit(1)
	}
	fmt.Printf("Updated %s with latest router methods.\n", readmePath)
}

func replaceBetweenMarkers(content, startMarker, endMarker, replacement string) (string, error) {
	startIndex := strings.Index(content, startMarker)
	if startIndex == -1 {
		return "", fmt.Errorf("marker %q not found", startMarker)
	}
	endIndex := strings.Index(content, endMarker)
	if endIndex == -1 {
		return "", fmt.Errorf("marker %q not found", endMarker)
	}
	if endIndex < startIndex {
		return "", fmt.Errorf("marker %q appears before %q", endMarker, startMarker)
	}

	before := content[:startIndex+len(startMarker)]
	after := content[endIndex:]
	return before + replacement + after, nil
}

func findRepoRoot() (string, error) {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(currentDirectory, "go.mod")); err == nil {
			return currentDirectory, nil
		}
		parentDirectory := filepath.Dir(currentDirectory)
		if parentDirectory == currentDirectory {
			break
		}
		currentDirectory = parentDirectory
	}
	return "", fmt.Errorf("could not find repo root (containing go.mod)")
}

func init() {
	_ = reflect.TypeOf(core.Kernel{})
}
