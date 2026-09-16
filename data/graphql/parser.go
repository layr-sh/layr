// Package graphql provides a single-shot GraphQL engine with schema introspection and caching.
package graphql

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// OperationType represents GraphQL operation (query or mutation).
type OperationType string

const (
	// QueryOperationType indicates a GraphQL query operation.
	QueryOperationType OperationType = "query"
	// MutationOperationType indicates a GraphQL mutation operation.
	MutationOperationType OperationType = "mutation"
)

// FieldNode represents a selected GraphQL field or relation.
type FieldNode struct {
	Name         string
	Alias        string
	Arguments    map[string]any
	SelectionSet []FieldNode
}

// OperationNode represents a parsed GraphQL operation document.
type OperationNode struct {
	Type           OperationType
	Name           string
	CacheTTL       int
	CacheKeySuffix string
	SelectionSet   []FieldNode
}

// Parser tokenizes and parses a GraphQL query string.
type Parser struct {
	input    []rune
	position int
	length   int
}

// ParseGraphQL parses a GraphQL query string into an OperationNode.
func ParseGraphQL(query string, variables map[string]any) (*OperationNode, error) {
	return ParseGraphQLWithOperation(query, "", variables)
}

// ParseGraphQLWithOperation parses a GraphQL query string and selects the operation matching targetOperationName.
func ParseGraphQLWithOperation(query string, targetOperationName string, variables map[string]any) (*OperationNode, error) {
	parser := &Parser{
		input:    []rune(query),
		position: 0,
		length:   len([]rune(query)),
	}

	parser.skipWhitespaceAndComments()
	if parser.isEOF() {
		return nil, fmt.Errorf("empty GraphQL query document")
	}

	var matchedOperationNode *OperationNode

	for !parser.isEOF() {
		operationName := ""
		var cacheTTL int
		var cacheKeySuffix string

		// Check if operation keyword is present
		operationType := QueryOperationType
		token := parser.peekIdent()
		if token == "subscription" {
			return nil, fmt.Errorf("subscriptions are not supported over HTTP GraphQL endpoint")
		}
		if token == "query" || token == "mutation" {
			parser.readIdent()
			if token == "mutation" {
				operationType = MutationOperationType
			}
			parser.skipWhitespaceAndComments()

			// Optional operation name
			if parser.peekIdent() != "" && parser.peekChar() != '{' && parser.peekChar() != '(' && parser.peekChar() != '@' {
				operationName = parser.readIdent()
				parser.skipWhitespaceAndComments()
			}

			// Optional variable definitions: ($var: Type = "default (val)")
			if parser.peekChar() == '(' {
				insideStringLiteral := false
				parenDepth := 0
				for !parser.isEOF() {
					charValue := parser.peekChar()
					if charValue == '"' && (parser.position == 0 || parser.input[parser.position-1] != '\\') {
						insideStringLiteral = !insideStringLiteral
					} else if !insideStringLiteral {
						if charValue == '(' {
							parenDepth++
						} else if charValue == ')' {
							parenDepth--
							if parenDepth == 0 {
								parser.position++ // consume closing ')'
								break
							}
						}
					}
					parser.position++
				}
				parser.skipWhitespaceAndComments()
			}

			// Optional directives on operation: @cache(ttl: 60, key_suffix: "custom_suffix")
			for parser.peekChar() == '@' {
				parser.position++ // consume '@'
				directiveName := parser.readIdent()
				parser.skipWhitespaceAndComments()
				var directiveArgs map[string]any
				if parser.peekChar() == '(' {
					args, err := parser.parseArguments(variables)
					if err != nil {
						return nil, err
					}
					directiveArgs = args
				}
				if directiveName == "cache" && directiveArgs != nil {
					if ttlParam, ok := directiveArgs["ttl"]; ok {
						switch typedValue := ttlParam.(type) {
						case int64:
							cacheTTL = int(typedValue)
						case int:
							cacheTTL = typedValue
						case float64:
							cacheTTL = int(typedValue)
						}
					}
					if suffixParam, ok := directiveArgs["key_suffix"].(string); ok {
						cacheKeySuffix = suffixParam
					} else if keyParam, ok := directiveArgs["key"].(string); ok {
						cacheKeySuffix = keyParam
					}
				}
				parser.skipWhitespaceAndComments()
			}
		}

		if parser.peekChar() != '{' {
			return nil, fmt.Errorf("expected '{' at position %d, found '%c'", parser.position, parser.peekChar())
		}

		fields, err := parser.parseSelectionSet(variables)
		if err != nil {
			return nil, err
		}

		operationNode := &OperationNode{
			Type:           operationType,
			Name:           operationName,
			CacheTTL:       cacheTTL,
			CacheKeySuffix: cacheKeySuffix,
			SelectionSet:   fields,
		}

		if targetOperationName != "" && operationName == targetOperationName {
			matchedOperationNode = operationNode
			break
		}

		// If anonymous query or target operation name is empty, we break after first operation
		if targetOperationName == "" {
			matchedOperationNode = operationNode
			break
		}
		parser.skipWhitespaceAndComments()
	}

	if targetOperationName != "" && matchedOperationNode == nil {
		return nil, fmt.Errorf("unknown operation %q in GraphQL document", targetOperationName)
	}

	return matchedOperationNode, nil
}

// CalculateDepth returns the maximum nested depth of the query.
func (operationNode *OperationNode) CalculateDepth() int {
	return maxFieldsDepth(operationNode.SelectionSet, 1)
}

func maxFieldsDepth(fields []FieldNode, currentDepth int) int {
	maxDepth := currentDepth
	for _, field := range fields {
		if len(field.SelectionSet) > 0 {
			childDepth := maxFieldsDepth(field.SelectionSet, currentDepth+1)
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	}
	return maxDepth
}

// CalculateComplexity computes the complexity cost of the query.
func (operationNode *OperationNode) CalculateComplexity() int {
	return computeFieldsComplexity(operationNode.SelectionSet, 1)
}

func computeFieldsComplexity(fields []FieldNode, multiplier int) int {
	total := 0
	for _, field := range fields {
		if len(field.SelectionSet) > 0 {
			// Nested join has weight 10 * multiplier
			total += 10 * multiplier
			total += computeFieldsComplexity(field.SelectionSet, multiplier*2)
		} else {
			// Scalar field has cost 1
			total += 1 * multiplier
		}
	}
	return total
}

func (parser *Parser) parseSelectionSet(variables map[string]any) ([]FieldNode, error) {
	if parser.peekChar() != '{' {
		return nil, fmt.Errorf("expected '{' at position %d", parser.position)
	}
	parser.position++ // consume '{'
	parser.skipWhitespaceAndComments()

	var fields []FieldNode
	for !parser.isEOF() && parser.peekChar() != '}' {
		parser.skipWhitespaceAndComments()

		fieldName := parser.readIdent()
		if fieldName == "" {
			return nil, fmt.Errorf("expected field name at position %d", parser.position)
		}
		parser.skipWhitespaceAndComments()

		alias := ""
		if parser.peekChar() == ':' {
			parser.position++ // consume ':'
			parser.skipWhitespaceAndComments()
			actualName := parser.readIdent()
			if actualName == "" {
				return nil, fmt.Errorf("expected field name after alias at position %d", parser.position)
			}
			alias = fieldName
			fieldName = actualName
			parser.skipWhitespaceAndComments()
		}

		args := make(map[string]any)
		if parser.peekChar() == '(' {
			parsedArgs, err := parser.parseArguments(variables)
			if err != nil {
				return nil, err
			}
			args = parsedArgs
			parser.skipWhitespaceAndComments()
		}

		var subFields []FieldNode
		if parser.peekChar() == '{' {
			children, err := parser.parseSelectionSet(variables)
			if err != nil {
				return nil, err
			}
			subFields = children
			parser.skipWhitespaceAndComments()
		}

		fields = append(fields, FieldNode{
			Name:         fieldName,
			Alias:        alias,
			Arguments:    args,
			SelectionSet: subFields,
		})
	}

	if parser.peekChar() != '}' {
		return nil, fmt.Errorf("expected '}' at position %d", parser.position)
	}
	parser.position++ // consume '}'
	parser.skipWhitespaceAndComments()

	return fields, nil
}

func (parser *Parser) parseArguments(variables map[string]any) (map[string]any, error) {
	if parser.peekChar() != '(' {
		return nil, fmt.Errorf("expected '(' at position %d", parser.position)
	}
	parser.position++ // consume '('
	parser.skipWhitespaceAndComments()

	args := make(map[string]any)
	for !parser.isEOF() && parser.peekChar() != ')' {
		parser.skipWhitespaceAndComments()

		argName := parser.readIdent()
		if argName == "" {
			return nil, fmt.Errorf("expected argument name at position %d", parser.position)
		}
		parser.skipWhitespaceAndComments()

		if parser.peekChar() != ':' {
			return nil, fmt.Errorf("expected ':' after argument name at position %d", parser.position)
		}
		parser.position++ // consume ':'
		parser.skipWhitespaceAndComments()

		parsedValue, err := parser.parseValue(variables)
		if err != nil {
			return nil, err
		}
		args[argName] = parsedValue

		parser.skipWhitespaceAndComments()
	}

	if parser.peekChar() != ')' {
		return nil, fmt.Errorf("expected ')' at position %d", parser.position)
	}
	parser.position++ // consume ')'

	if len(args) == 0 {
		return nil, fmt.Errorf("empty argument list at position %d", parser.position)
	}

	return args, nil
}

//nolint:nilnil
func (parser *Parser) parseValue(variables map[string]any) (any, error) {
	parser.skipWhitespaceAndComments()
	peeked := parser.peekChar()

	if peeked == '$' {
		parser.position++ // consume '$'
		varName := parser.readIdent()
		if resolvedValue, ok := variables[varName]; ok {
			return resolvedValue, nil
		}
		return nil, nil // unsupplied variable
	}

	if peeked == '"' {
		return parser.readString()
	}

	if peeked == '{' {
		parser.position++ // consume '{'
		objectMap := make(map[string]any)
		for !parser.isEOF() && parser.peekChar() != '}' {
			parser.skipWhitespaceAndComments()
			fieldName := parser.readIdent()
			parser.skipWhitespaceAndComments()
			if parser.peekChar() != ':' {
				return nil, fmt.Errorf("expected ':' in object at position %d", parser.position)
			}
			parser.position++
			elementValue, err := parser.parseValue(variables)
			if err != nil {
				return nil, err
			}
			objectMap[fieldName] = elementValue
			parser.skipWhitespaceAndComments()
		}
		if parser.peekChar() != '}' {
			return nil, fmt.Errorf("expected '}' in object at position %d", parser.position)
		}
		parser.position++
		return objectMap, nil
	}

	if peeked == '[' {
		parser.position++ // consume '['
		var arrayItems []any
		for !parser.isEOF() && parser.peekChar() != ']' {
			parser.skipWhitespaceAndComments()
			element, err := parser.parseValue(variables)
			if err != nil {
				return nil, err
			}
			arrayItems = append(arrayItems, element)
			parser.skipWhitespaceAndComments()
		}
		if parser.peekChar() != ']' {
			return nil, fmt.Errorf("expected ']' in array at position %d", parser.position)
		}
		parser.position++
		return arrayItems, nil
	}

	if (peeked >= '0' && peeked <= '9') || peeked == '-' {
		return parser.readNumber()
	}

	ident := parser.readIdent()
	if ident == "" {
		return nil, fmt.Errorf("unexpected character '%c' at position %d", peeked, parser.position)
	}
	if ident == "true" {
		return true, nil
	}
	if ident == "false" {
		return false, nil
	}
	if ident == "null" {
		return nil, nil
	}

	return ident, nil
}

func (parser *Parser) readString() (string, error) {
	parser.position++ // consume opening '"'
	var builder strings.Builder
	for !parser.isEOF() {
		current := parser.input[parser.position]
		parser.position++
		if current == '"' {
			return builder.String(), nil
		}
		if current == '\\' && !parser.isEOF() {
			escaped := parser.input[parser.position]
			parser.position++
			switch escaped {
			case 'n':
				builder.WriteRune('\n')
			case 't':
				builder.WriteRune('\t')
			case 'r':
				builder.WriteRune('\r')
			case 'b':
				builder.WriteRune('\b')
			case 'f':
				builder.WriteRune('\f')
			case '"':
				builder.WriteRune('"')
			case '\\':
				builder.WriteRune('\\')
			case '/':
				builder.WriteRune('/')
			default:
				builder.WriteRune(escaped)
			}
		} else {
			builder.WriteRune(current)
		}
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (parser *Parser) readNumber() (any, error) {
	start := parser.position
	isFloat := false
	for !parser.isEOF() {
		current := parser.input[parser.position]
		if (current >= '0' && current <= '9') || current == '-' {
			parser.position++
		} else if current == '.' {
			isFloat = true
			parser.position++
		} else {
			break
		}
	}
	numberText := string(parser.input[start:parser.position])
	if isFloat {
		parsedFloat, err := strconv.ParseFloat(numberText, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse float: %w", err)
		}
		return parsedFloat, nil
	}
	parsedInteger, err := strconv.ParseInt(numberText, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse int: %w", err)
	}
	return parsedInteger, nil
}

func (parser *Parser) readIdent() string {
	start := parser.position
	for !parser.isEOF() {
		currentChar := parser.input[parser.position]
		if unicode.IsLetter(currentChar) || unicode.IsDigit(currentChar) || currentChar == '_' {
			parser.position++
		} else {
			break
		}
	}
	return string(parser.input[start:parser.position])
}

func (parser *Parser) peekIdent() string {
	orig := parser.position
	ident := parser.readIdent()
	parser.position = orig
	return ident
}

func (parser *Parser) peekChar() rune {
	if parser.isEOF() {
		return 0
	}
	return parser.input[parser.position]
}

func (parser *Parser) isEOF() bool {
	return parser.position >= parser.length
}

func (parser *Parser) skipWhitespaceAndComments() {
	for !parser.isEOF() {
		currentChar := parser.input[parser.position]
		if unicode.IsSpace(currentChar) || currentChar == ',' {
			parser.position++
		} else if currentChar == '#' {
			for !parser.isEOF() && parser.input[parser.position] != '\n' {
				parser.position++
			}
		} else {
			break
		}
	}
}
