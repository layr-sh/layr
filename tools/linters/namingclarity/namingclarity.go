package namingclarity

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin(analyzerName, New)
}

func New(conf any) (register.LinterPlugin, error) {
	return &plugin{}, nil
}

type plugin struct{}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

const (
	analyzerName   = "namingclarity"
	defaultMessage = "variable, argument, and test function naming must be clear and comply with repository conventions"
)

var Analyzer = &analysis.Analyzer{
	Name: analyzerName,
	Doc:  defaultMessage,
	Run:  run,
}

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

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				checkTestFunctionName(pass, n)

			case *ast.AssignStmt:
				checkAssignment(pass, n)

			case *ast.ValueSpec:
				checkValueSpec(pass, n)

			case *ast.RangeStmt:
				checkRange(pass, n)

			case *ast.StructType:
				if n.Fields != nil {
					checkStructFields(pass, n.Fields.List)
				}

			case *ast.FuncType:
				if n.Params != nil {
					checkFuncParams(pass, n.Params)
					checkFields(pass, n.Params.List)
				}
				if n.Results != nil {
					checkFields(pass, n.Results.List)
				}
			}

			return true
		})
	}

	return nil, nil
}

func toPascalCase(input string) string {
	parts := strings.FieldsFunc(input, func(character rune) bool {
		return character == '_' || character == '-' || character == '.' || character == ' ' || character == '/'
	})
	var builder strings.Builder
	for _, part := range parts {
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

func checkTestFunctionName(pass *analysis.Pass, funcDecl *ast.FuncDecl) {
	if funcDecl == nil || funcDecl.Name == nil || funcDecl.Type == nil {
		return
	}

	functionName := funcDecl.Name.Name
	if !strings.HasPrefix(functionName, "Test") {
		return
	}

	tokenFile := pass.Fset.File(funcDecl.Pos())
	if tokenFile == nil {
		return
	}

	filePath := tokenFile.Name()
	if !strings.HasSuffix(filePath, "_test.go") {
		return
	}

	// Skip ignored directories
	normalizedPath := filepath.ToSlash(filePath)
	for _, part := range strings.Split(normalizedPath, "/") {
		if skippedDirectories[part] {
			return
		}
	}

	absPath, err := filepath.Abs(filePath)
	if err == nil {
		filePath = absPath
	}

	folderName := filepath.Base(filepath.Dir(filePath))
	expectedPrefixes := allowedPrefixesForFolder(folderName)

	if !testNamePatterns.MatchString(functionName) {
		pass.Reportf(
			funcDecl.Name.Pos(),
			"violates convention 'Test<Subject><Scenario><Unit|Integration|E2E>'",
		)
		return
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
		pass.Reportf(
			funcDecl.Name.Pos(),
			"test function name in folder '%s' must start with '%s'",
			folderName,
			prefixLabel,
		)
		return
	}

	switch {
	case strings.HasSuffix(filePath, "_e2e_test.go"):
		if !strings.HasSuffix(functionName, "E2E") {
			pass.Reportf(
				funcDecl.Name.Pos(),
				"test in e2e file must end with 'E2E'",
			)
		}
	case strings.HasSuffix(filePath, "_integration_test.go"):
		if !strings.HasSuffix(functionName, "Integration") {
			pass.Reportf(
				funcDecl.Name.Pos(),
				"test in integration file must end with 'Integration'",
			)
		}
	default:
		if !strings.HasSuffix(functionName, "Unit") {
			pass.Reportf(
				funcDecl.Name.Pos(),
				"test in unit test file must end with 'Unit'",
			)
		}
	}

	checkTestTableSubtests(pass, funcDecl)
}

func checkTestTableSubtests(pass *analysis.Pass, funcDecl *ast.FuncDecl) {
	if funcDecl.Body == nil {
		return
	}

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}

		rangeStmt, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}

		hasAssertions := false
		hasRun := false
		hasBreak := false

		ast.Inspect(rangeStmt.Body, func(child ast.Node) bool {
			if branch, isBranch := child.(*ast.BranchStmt); isBranch && branch.Tok == token.BREAK {
				hasBreak = true
			}

			callExpr, isCall := child.(*ast.CallExpr)
			if !isCall {
				return true
			}

			selExpr, isSel := callExpr.Fun.(*ast.SelectorExpr)
			if !isSel {
				return true
			}

			if selExpr.Sel.Name == "Run" {
				hasRun = true
			}

			methodName := selExpr.Sel.Name
			if methodName == "Fatalf" || methodName == "Errorf" || methodName == "Fatal" || methodName == "Error" || methodName == "Fail" || methodName == "FailNow" {
				hasAssertions = true
			}

			if ident, isIdent := selExpr.X.(*ast.Ident); isIdent {
				if ident.Name == "assert" || ident.Name == "require" {
					hasAssertions = true
				}
			}

			return true
		})

		if hasAssertions && !hasRun && !hasBreak {
			pass.Reportf(
				rangeStmt.Pos(),
				"table-driven test loop in '%s' must execute cases using t.Run",
				funcDecl.Name.Name,
			)
		}

		return true
	})
}

func inspectIdentifier(pass *analysis.Pass, identifier *ast.Ident, concreteName string) bool {
	if checkProhibitedSuffix(pass, identifier) {
		return true
	}
	if concreteName != "" && checkIdentifier(pass, identifier, concreteName) {
		return true
	}
	return checkDisallowedIdentifier(pass, identifier, concreteName)
}

func checkFuncParams(pass *analysis.Pass, params *ast.FieldList) {
	if params == nil {
		return
	}

	for _, field := range params.List {
		var concreteName string
		if field.Type != nil {
			concreteName, _ = concreteTypeName(pass.TypesInfo.TypeOf(field.Type))
		}

		for _, identifier := range field.Names {
			if len(identifier.Name) == 1 && identifier.Name != "_" {
				if isTestingT(pass, identifier) {
					continue
				}

				pass.Reportf(
					identifier.Pos(),
					"single-letter argument prohibited: %s",
					identifier.Name,
				)
				continue
			}

			inspectIdentifier(pass, identifier, concreteName)
		}
	}
}

func isTestingT(pass *analysis.Pass, identifier *ast.Ident) bool {
	if identifier.Name != "t" {
		return false
	}
	if typeValue := pass.TypesInfo.TypeOf(identifier); typeValue != nil {
		if strings.Contains(typeValue.String(), "testing.") {
			return true
		}
	}
	file := pass.Fset.File(identifier.Pos())
	return file != nil && strings.HasSuffix(file.Name(), "_test.go")
}

func isExemptIdentifier(pass *analysis.Pass, identifier *ast.Ident, concreteName string) bool {
	name := identifier.Name
	if name == "_" || name == "ok" {
		return true
	}

	if isTestingT(pass, identifier) {
		return true
	}

	if rule, ok := findSpecialTypeRule(concreteName); ok {
		if rule.checkAllowed(name) {
			return true
		}
	}

	return false
}

func checkRange(pass *analysis.Pass, rangeStmt *ast.RangeStmt) {
	if key, ok := rangeStmt.Key.(*ast.Ident); ok {
		inspectIdentifier(pass, key, "")
	}
	if value, ok := rangeStmt.Value.(*ast.Ident); ok {
		inspectIdentifier(pass, value, "")
	}
}

func checkAssignment(pass *analysis.Pass, assignStmt *ast.AssignStmt) {
	for _, lhsExpr := range assignStmt.Lhs {
		if identifier, ok := lhsExpr.(*ast.Ident); ok {
			checkProhibitedSuffix(pass, identifier)
		}
	}

	if len(assignStmt.Lhs) == len(assignStmt.Rhs) {
		for index, rhs := range assignStmt.Rhs {
			identifier, ok := assignStmt.Lhs[index].(*ast.Ident)
			if !ok || hasProhibitedSuffix(identifier.Name) {
				continue
			}

			concreteName, _ := concreteTypeNameFromExpression(pass, rhs)
			if concreteName == "" {
				concreteName, _ = concreteTypeNameFromIdent(pass, identifier)
			}

			inspectIdentifier(pass, identifier, concreteName)
		}
		return
	}

	if len(assignStmt.Rhs) == 1 && len(assignStmt.Lhs) > 1 {
		rhsType := pass.TypesInfo.TypeOf(assignStmt.Rhs[0])
		if tuple, ok := rhsType.(*types.Tuple); ok && tuple.Len() == len(assignStmt.Lhs) {
			for index, lhsExpr := range assignStmt.Lhs {
				identifier, ok := lhsExpr.(*ast.Ident)
				if !ok || hasProhibitedSuffix(identifier.Name) {
					continue
				}

				concreteName, _ := concreteTypeName(tuple.At(index).Type())
				inspectIdentifier(pass, identifier, concreteName)
			}
			return
		}

		for _, lhsExpr := range assignStmt.Lhs {
			identifier, ok := lhsExpr.(*ast.Ident)
			if !ok || hasProhibitedSuffix(identifier.Name) {
				continue
			}
			concreteName, _ := concreteTypeNameFromIdent(pass, identifier)
			inspectIdentifier(pass, identifier, concreteName)
		}
	}
}

func checkValueSpec(pass *analysis.Pass, valueSpec *ast.ValueSpec) {
	var typeConcreteName string
	var hasTypeConcreteName bool
	if valueSpec.Type != nil {
		typeConcreteName, hasTypeConcreteName = concreteTypeName(pass.TypesInfo.TypeOf(valueSpec.Type))
	}

	for index, identifier := range valueSpec.Names {

		concreteName := typeConcreteName
		ok := hasTypeConcreteName

		if !ok && index < len(valueSpec.Values) {
			concreteName, ok = concreteTypeNameFromExpression(pass, valueSpec.Values[index])
		}

		if !ok {
			concreteName, _ = concreteTypeNameFromIdent(pass, identifier)
		}

		inspectIdentifier(pass, identifier, concreteName)
	}
}

func checkStructFields(pass *analysis.Pass, fields []*ast.Field) {
	for _, field := range fields {
		if len(field.Names) == 0 {
			continue
		}

		var concreteName string
		var ok bool

		if field.Type != nil {
			concreteName, ok = concreteTypeName(pass.TypesInfo.TypeOf(field.Type))
		}

		for _, identifier := range field.Names {
			// Exported struct fields in Go represent public API or serialized model fields
			// (e.g. Actor, Kind, MFA, OIDC). They should not be forced into lowercase canonical variable names.
			if len(identifier.Name) > 0 && unicode.IsUpper(rune(identifier.Name[0])) {
				continue
			}

			identConcreteName := concreteName
			if !ok {
				identConcreteName, _ = concreteTypeNameFromIdent(pass, identifier)
			}

			inspectIdentifier(pass, identifier, identConcreteName)
		}
	}
}

func checkFields(pass *analysis.Pass, fields []*ast.Field) {
	for _, field := range fields {
		if len(field.Names) == 0 {
			continue
		}

		var concreteName string
		var ok bool

		if field.Type != nil {
			concreteName, ok = concreteTypeName(pass.TypesInfo.TypeOf(field.Type))
		}

		for _, identifier := range field.Names {
			identConcreteName := concreteName
			if !ok {
				identConcreteName, _ = concreteTypeNameFromIdent(pass, identifier)
			}

			inspectIdentifier(pass, identifier, identConcreteName)
		}
	}
}

func hasProhibitedSuffix(name string) bool {
	if name == "_" {
		return false
	}

	// BigInt / BigInteger represents math/big.Int, not Hungarian notation "Int".
	if strings.HasSuffix(name, "BigInt") || strings.HasSuffix(name, "bigInt") ||
		strings.HasSuffix(name, "BigInteger") || strings.HasSuffix(name, "bigInteger") {
		return false
	}

	for _, suffix := range prohibitedSuffixes {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			return true
		}
	}

	return false
}

func checkProhibitedSuffix(pass *analysis.Pass, identifier *ast.Ident) bool {
	if identifier == nil || identifier.Name == "_" {
		return false
	}

	if hasProhibitedSuffix(identifier.Name) {
		pass.Reportf(
			identifier.Pos(),
			"meaningless representation suffix prohibited: %s",
			identifier.Name,
		)
		return true
	}

	return false
}

func isDisallowedIdentifier(name string) (string, bool) {
	if name == "" || name == "_" {
		return "", false
	}

	lower := strings.ToLower(name)

	// 1. Check compound patterns.
	for _, pattern := range disallowedCompoundPatterns {
		if strings.Contains(lower, pattern) {
			return pattern, true
		}
	}

	// 2. Check individual component words.
	words := splitIdentifierWords(name)
	for i, word := range words {
		if badTerm, ok := badInitialisms[word]; ok {
			return badTerm, true
		}
		wordLower := strings.ToLower(word)
		if wordLower == "pool" {
			// Allow "CertPool" / "CertificatePool" component (e.g. caCertPool, rootCertPool, certPool)
			if i > 0 && (strings.EqualFold(words[i-1], "cert") || strings.EqualFold(words[i-1], "certificate")) {
				continue
			}
			// Allow "DatabasePoolOptions" / "DatabasePoolOption" component
			if i > 0 && strings.EqualFold(words[i-1], "database") && i+1 < len(words) && (strings.EqualFold(words[i+1], "options") || strings.EqualFold(words[i+1], "option")) {
				continue
			}
		}
		if disallowedWords[wordLower] {
			return wordLower, true
		}
	}

	return "", false
}

func checkDisallowedIdentifier(pass *analysis.Pass, identifier *ast.Ident, concreteName ...string) bool {
	if identifier == nil || identifier.Name == "_" {
		return false
	}

	actualConcreteName := ""
	if len(concreteName) > 0 {
		actualConcreteName = concreteName[0]
	}

	term, isDisallowed := isDisallowedIdentifier(identifier.Name)
	if !isDisallowed {
		return false
	}

	// The word "pool" is allowed when representing sync.Pool or pgxpool.Pool
	if term == "pool" && (actualConcreteName == "pgxpool.Pool" || actualConcreteName == "sync.Pool") {
		return false
	}

	pass.Reportf(
		identifier.Pos(),
		"disallowed identifier term %q prohibited: %s",
		term,
		identifier.Name,
	)
	return true
}

func splitIdentifierWords(name string) []string {
	var words []string
	var current []rune
	runes := []rune(name)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '_' || unicode.IsDigit(r) {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
			continue
		}
		if unicode.IsUpper(r) {
			if len(current) > 0 {
				prevIsUpper := unicode.IsUpper(runes[i-1])
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if !prevIsUpper || nextIsLower {
					words = append(words, string(current))
					current = nil
				}
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

func typeNamingTargets(concreteName string) (string, string) {
	if rule, ok := findSpecialTypeRule(concreteName); ok {
		return rule.canonicalName, rule.expectedTail
	}
	bare := bareTypeName(concreteName)
	return canonicalVariableName(bare), capitalizeFirst(bare)
}

func shouldReportUnclearName(name string, concreteName string) (string, bool) {
	if name == "" || name == "_" || concreteName == "" {
		return "", false
	}

	// 1. Consult declarative special type rules
	if rule, ok := findSpecialTypeRule(concreteName); ok {
		if rule.checkAllowed(name) {
			return "", false
		}
		if concreteName == "error" || concreteName == "Error" {
			return fmt.Sprintf("%q should be named %q, suffixed with %q, or prefixed with %q", name, "err", "Err", "Err"), true
		}
		return fmt.Sprintf("%q should be named %q or suffixed with %q", name, rule.canonicalName, rule.expectedTail), true
	}

	bare := bareTypeName(concreteName)
	canonicalName, expectedTail := typeNamingTargets(concreteName)

	// 2. Case-insensitive exact match (e.g. oAuthStatePayload for OAuthStatePayload)
	if strings.EqualFold(name, bare) || strings.EqualFold(name, canonicalName) {
		return "", false
	}

	// 3. Allowed if name ends with concrete type name or canonical name (allowing plural 's')
	nameSingular := strings.TrimSuffix(name, "s")
	if strings.HasSuffix(name, bare) || strings.HasSuffix(name, expectedTail) || strings.HasSuffix(strings.ToLower(name), strings.ToLower(bare)) ||
		strings.HasSuffix(nameSingular, bare) || strings.HasSuffix(nameSingular, expectedTail) || strings.HasSuffix(strings.ToLower(nameSingular), strings.ToLower(bare)) {
		return "", false
	}

	// 4. For unexported types: allowed if name ends with or equals an uppercase suffix
	// e.g. slogHandler allows handler, baseRoute allows route
	if unicode.IsLower(rune(bare[0])) {
		for i := 1; i < len(bare); i++ {
			if unicode.IsUpper(rune(bare[i])) {
				subSuffix := bare[i:]
				if strings.HasSuffix(name, subSuffix) || strings.EqualFold(name, subSuffix) {
					return "", false
				}
			}
		}
	}

	return fmt.Sprintf("%q should be named %q or suffixed with %q", name, canonicalName, expectedTail), true
}

func checkIdentifier(
	pass *analysis.Pass,
	identifier *ast.Ident,
	concreteName string,
) bool {
	if isExemptIdentifier(pass, identifier, concreteName) {
		return false
	}

	reportMessage, shouldReport := shouldReportUnclearName(identifier.Name, concreteName)
	if !shouldReport {
		return false
	}

	pass.Reportf(identifier.Pos(), "%s", reportMessage)
	return true
}

func concreteTypeNameFromExpression(
	pass *analysis.Pass,
	expr ast.Expr,
) (string, bool) {
	typeValue := pass.TypesInfo.TypeOf(expr)
	if typeValue == nil {
		return "", false
	}

	return concreteTypeName(typeValue)
}

func concreteTypeNameFromIdent(
	pass *analysis.Pass,
	identifier *ast.Ident,
) (string, bool) {
	if object := pass.TypesInfo.Defs[identifier]; object != nil && object.Type() != nil {
		return concreteTypeName(object.Type())
	}
	if object := pass.TypesInfo.Uses[identifier]; object != nil && object.Type() != nil {
		return concreteTypeName(object.Type())
	}
	if typeValue := pass.TypesInfo.TypeOf(identifier); typeValue != nil {
		return concreteTypeName(typeValue)
	}
	return "", false
}

func concreteTypeName(typeValue types.Type) (string, bool) {
	if typeValue == nil {
		return "", false
	}

	// Remove pointers.
	for {
		pointer, ok := typeValue.(*types.Pointer)
		if !ok {
			break
		}

		typeValue = pointer.Elem()
	}

	// Handle named types.
	if named, ok := typeValue.(*types.Named); ok {
		typeName := named.Obj().Name()
		if pkg := named.Obj().Pkg(); pkg != nil {
			return pkg.Name() + "." + typeName, true
		}
		return typeName, true
	}

	// Handle aliases.
	if alias, ok := typeValue.(*types.Alias); ok {
		return concreteTypeName(alias.Rhs())
	}

	return "", false
}

func canonicalVariableName(concreteName string) string {
	if rule, ok := findSpecialTypeRule(concreteName); ok && rule.canonicalName != "" {
		return rule.canonicalName
	}
	return lowerPrefix(bareTypeName(concreteName))
}

func capitalizeFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func lowerPrefix(value string) string {
	if value == "" {
		return value
	}

	// 1. Check if the value starts with any known initialism.
	for _, initialism := range knownInitialisms {
		if strings.HasPrefix(value, initialism) {
			remainder := value[len(initialism):]
			if remainder == "" {
				return strings.ToLower(initialism)
			}
			if unicode.IsUpper(rune(remainder[0])) {
				return strings.ToLower(initialism) + remainder
			}
		}
	}

	runes := []rune(value)

	// 2. If all runes are uppercase, lowercase all of them.
	allUpper := true
	for _, r := range runes {
		if !unicode.IsUpper(r) {
			allUpper = false
			break
		}
	}
	if allUpper {
		return strings.ToLower(value)
	}

	// 3. If prefix starts with consecutive uppercase runes followed by PascalCase,
	// lowercase the leading acronym up to the last uppercase rune before a lowercase rune.
	if len(runes) > 1 && unicode.IsUpper(runes[0]) && unicode.IsUpper(runes[1]) {
		i := 0
		for i < len(runes) && unicode.IsUpper(runes[i]) {
			if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				break
			}
			i++
		}
		for j := 0; j < i; j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		return string(runes)
	}

	// 4. Standard PascalCase: lowercase only the first rune.
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
