// Package main provides a standalone CLI tool to verify test naming conventions across the repository.
package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// testNamePatterns enforces Test<Subject><Scenario><Tier> where Tier is Unit, Integration, or E2E.
	testNamePatterns = regexp.MustCompile(`^Test[A-Z][a-zA-Z0-9]+(Unit|Integration|E2E)$`)

	// skippedDirectories are directory names ignored during repository traversal.
	skippedDirectories = map[string]bool{
		".cache":       true,
		".git":         true,
		".githooks":    true,
		".vscode":      true,
		".agents":      true,
		".gemini":      true,
		".local":       true,
		"vendor":       true,
		"node_modules": true,
		"bin":          true,
		"dist":         true,
	}
)

// Violation represents an identified test naming rule infraction.
type Violation struct {
	Position     token.Position
	FunctionName string
	FilePath     string
	Message      string
}

func findRepositoryRoot() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %w", err)
	}

	currentDirectory := workingDirectory
	for {
		candidateGoMod := filepath.Join(currentDirectory, "go.mod")
		if _, statErr := os.Stat(candidateGoMod); statErr == nil {
			return currentDirectory, nil
		}

		parentDirectory := filepath.Dir(currentDirectory)
		if parentDirectory == currentDirectory {
			return "", errors.New("repository root with go.mod not found")
		}
		currentDirectory = parentDirectory
	}
}

func toPascalCase(input string) string {
	parts := strings.FieldsFunc(input, func(character rune) bool {
		return character == '_' || character == '-' || character == '.' || character == ' ' || character == '/'
	})
	var builder strings.Builder
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

func allowedPrefixesForFolder(folderName string) []string {
	folderPascal := toPascalCase(folderName)
	standardPrefix := "Test" + folderPascal
	uppercasePrefix := "Test" + strings.ToUpper(folderPascal)
	if standardPrefix == uppercasePrefix {
		return []string{standardPrefix}
	}
	return []string{uppercasePrefix, standardPrefix}
}

func inspectTestFile(fileSet *token.FileSet, filePath, repositoryRoot string) ([]Violation, int, error) {
	parsedFile, parseErr := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if parseErr != nil {
		return nil, 0, fmt.Errorf("failed to parse file %s: %w", filePath, parseErr)
	}

	relativeFilePath, relErr := filepath.Rel(repositoryRoot, filePath)
	if relErr != nil {
		relativeFilePath = filePath
	}

	folderName := filepath.Base(filepath.Dir(filePath))
	expectedPrefixes := allowedPrefixesForFolder(folderName)

	var violations []Violation
	testCount := 0

	for _, declaration := range parsedFile.Decls {
		functionDeclaration, isFunc := declaration.(*ast.FuncDecl)
		if !isFunc || !strings.HasPrefix(functionDeclaration.Name.Name, "Test") {
			continue
		}

		testCount++
		functionName := functionDeclaration.Name.Name
		sourcePosition := fileSet.Position(functionDeclaration.Pos())

		if !testNamePatterns.Match([]byte(functionName)) {
			violations = append(violations, Violation{
				Position:     sourcePosition,
				FunctionName: functionName,
				FilePath:     relativeFilePath,
				Message:      "violates convention 'Test<Subject><Scenario><Unit|Integration|E2E>'",
			})
			continue
		}

		hasValidPrefix := false
		for _, expectedPrefix := range expectedPrefixes {
			if strings.HasPrefix(functionName, expectedPrefix) {
				hasValidPrefix = true
				break
			}
		}

		if !hasValidPrefix {
			prefixLabel := expectedPrefixes[0]
			if len(expectedPrefixes) > 1 {
				prefixLabel = strings.Join(expectedPrefixes, "' or '")
			}
			violations = append(violations, Violation{
				Position:     sourcePosition,
				FunctionName: functionName,
				FilePath:     relativeFilePath,
				Message:      fmt.Sprintf("test function name in folder '%s' must start with '%s'", folderName, prefixLabel),
			})
			continue
		}

		switch {
		case strings.HasSuffix(filePath, "_e2e_test.go"):
			if !strings.HasSuffix(functionName, "E2E") {
				violations = append(violations, Violation{
					Position:     sourcePosition,
					FunctionName: functionName,
					FilePath:     relativeFilePath,
					Message:      "test in e2e file must end with 'E2E'",
				})
			}
		case strings.HasSuffix(filePath, "_integration_test.go"):
			if !strings.HasSuffix(functionName, "Integration") {
				violations = append(violations, Violation{
					Position:     sourcePosition,
					FunctionName: functionName,
					FilePath:     relativeFilePath,
					Message:      "test in integration file must end with 'Integration'",
				})
			}
		default:
			if !strings.HasSuffix(functionName, "Unit") {
				violations = append(violations, Violation{
					Position:     sourcePosition,
					FunctionName: functionName,
					FilePath:     relativeFilePath,
					Message:      "test in unit test file must end with 'Unit'",
				})
			}
		}
	}

	return violations, testCount, nil
}

func printUsage() {
	fmt.Println("Usage: check_test_names [flags] [paths...]")
	fmt.Println()
	fmt.Println("Enforces the Test<Subject><Scenario><Tier> naming convention.")
	fmt.Println("<Subject> must start with the enclosing directory name in PascalCase (e.g. TestCore... in core/).")
	fmt.Println("Tier must be Unit, Integration, or E2E, matching the file suffix:")
	fmt.Println("  *_test.go             -> Unit")
	fmt.Println("  *_integration_test.go -> Integration")
	fmt.Println("  *_e2e_test.go         -> E2E")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  paths   Optional directory or file paths to check (defaults to repository root)")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -q, --quiet  Suppress output on success (only print violations)")
	fmt.Println("  -h, --help   Show this help message")
}

func main() {
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	quietMode := false
	var targetPaths []string
	for _, arg := range os.Args[1:] {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" || trimmed == "--" {
			continue
		}
		if trimmed == "-q" || trimmed == "--quiet" {
			quietMode = true
			continue
		}
		if trimmed == "-h" || trimmed == "--help" {
			printUsage()
			return
		}
		targetPaths = append(targetPaths, trimmed)
	}

	if len(targetPaths) == 0 {
		targetPaths = []string{repositoryRoot}
	}

	fileSet := token.NewFileSet()
	var allViolations []Violation
	totalTestCount := 0

	for _, rawPath := range targetPaths {
		targetPath := rawPath
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(repositoryRoot, targetPath)
		}

		pathFileInfo, statErr := os.Stat(targetPath)
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "Error accessing target path %s: %v\n", targetPath, statErr)
			os.Exit(1)
		}

		if !pathFileInfo.IsDir() {
			if strings.HasSuffix(targetPath, "_test.go") {
				fileViolations, testCount, inspectErr := inspectTestFile(fileSet, targetPath, repositoryRoot)
				if inspectErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", inspectErr)
					os.Exit(1)
				}
				allViolations = append(allViolations, fileViolations...)
				totalTestCount += testCount
			}
			continue
		}

		walkErr := filepath.Walk(targetPath, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if fileInfo.IsDir() {
				if skippedDirectories[fileInfo.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			if !strings.HasSuffix(filePath, "_test.go") {
				return nil
			}

			fileViolations, testCount, inspectErr := inspectTestFile(fileSet, filePath, repositoryRoot)
			if inspectErr != nil {
				return inspectErr
			}
			allViolations = append(allViolations, fileViolations...)
			totalTestCount += testCount
			return nil
		})

		if walkErr != nil {
			fmt.Fprintf(os.Stderr, "Error scanning files in %s: %v\n", targetPath, walkErr)
			os.Exit(1)
		}
	}

	if len(allViolations) > 0 {
		fmt.Fprintf(os.Stderr, "error: %d test naming convention violation(s):\n", len(allViolations))
		for _, violation := range allViolations {
			fmt.Fprintf(os.Stderr, "  %s:%d:%d: function '%s' %s\n",
				violation.FilePath, violation.Position.Line, violation.Position.Column,
				violation.FunctionName, violation.Message)
		}
		fmt.Fprintln(os.Stderr, "\nrule: Test<Subject><Scenario><Tier> where <Subject> starts with enclosing folder name (Tier: Unit | Integration | E2E)")
		os.Exit(1)
	}

	if !quietMode {
		fmt.Printf("All %d test function(s) comply with naming conventions.\n", totalTestCount)
	}
}
