package rest

import (
	"fmt"
	"strconv"
	"strings"

	"layr.sh/data/common"
)

// FilterOp represents a parsed column filter.
type FilterOp struct {
	Column  string
	Op      string
	Value   string
	Extra   string // For fts language etc.
	Values  []string
	Negated bool
}

// OrderBy represents a sort instruction.
type OrderBy struct {
	Column    string
	Desc      bool
	NullsLast bool
}

// EmbeddedField represents a nested relation in ?select=.
type EmbeddedField struct {
	Relation string
	Fields   []string
	Children []EmbeddedField
}

// QueryParams represents all parsed query parameters from the request.
type QueryParams struct {
	Fields     []string
	Embedded   []EmbeddedField
	Filters    []FilterOp
	Orders     []OrderBy
	Limit      int
	Offset     int
	CountExact bool
	OnConflict string
}

// ParseQueryParams parses URL query parameters into a structured QueryParams object.
func ParseQueryParams(urlValues map[string][]string, defaultLimit, maxLimit int) (*QueryParams, error) {
	queryParams := &QueryParams{
		Limit:  defaultLimit,
		Offset: 0,
	}

	for rawKey, valueList := range urlValues {
		if len(valueList) == 0 {
			continue
		}
		firstValue := valueList[0]

		switch rawKey {
		case "select":
			fields, embedded, err := parseSelect(firstValue)
			if err != nil {
				return nil, fmt.Errorf("invalid select syntax: %w", err)
			}
			queryParams.Fields = fields
			queryParams.Embedded = embedded

		case "order":
			orders, err := parseOrder(firstValue)
			if err != nil {
				return nil, fmt.Errorf("invalid order syntax: %w", err)
			}
			queryParams.Orders = orders

		case "limit":
			parsedLimit, err := strconv.Atoi(firstValue)
			if err != nil || parsedLimit < 0 {
				return nil, fmt.Errorf("invalid limit parameter: %s", firstValue)
			}
			if parsedLimit > maxLimit {
				parsedLimit = maxLimit
			}
			queryParams.Limit = parsedLimit

		case "offset":
			parsedOffset, err := strconv.Atoi(firstValue)
			if err != nil || parsedOffset < 0 {
				return nil, fmt.Errorf("invalid offset parameter: %s", firstValue)
			}
			queryParams.Offset = parsedOffset

		case "count":
			if firstValue == "exact" {
				queryParams.CountExact = true
			}

		case "on_conflict":
			if !common.IsValidIdentifier(firstValue) {
				return nil, fmt.Errorf("invalid on_conflict identifier: %s", firstValue)
			}
			queryParams.OnConflict = firstValue

		case "cache_ttl", "cache_key", "cache_key_suffix":
			// Opt-in caching directives, ignore as column filter
			continue

		default:
			// Treat as column filter
			for _, rawFilterValue := range valueList {
				filterOp, err := parseFilter(rawKey, rawFilterValue)
				if err != nil {
					return nil, err
				}
				queryParams.Filters = append(queryParams.Filters, filterOp)
			}
		}
	}

	return queryParams, nil
}

func parseFilter(column, raw string) (FilterOp, error) {
	if !common.IsValidIdentifier(column) {
		return FilterOp{}, fmt.Errorf("invalid column name: %s", column)
	}

	negated := false
	trimmed := raw
	if strings.HasPrefix(trimmed, "not.") {
		negated = true
		trimmed = strings.TrimPrefix(trimmed, "not.")
	}

	dotIdx := strings.Index(trimmed, ".")
	if dotIdx == -1 {
		return FilterOp{}, fmt.Errorf("invalid filter format for %s: missing filter prefix (e.g. eq.value)", column)
	}

	filterOp := trimmed[:dotIdx]
	value := trimmed[dotIdx+1:]

	switch filterOp {
	case "eq", "neq", "gt", "gte", "lt", "lte", "like", "ilike":
		// replace wildcards if ilike/like uses *
		if filterOp == "ilike" || filterOp == "like" {
			value = strings.ReplaceAll(value, "*", "%")
		}
		return FilterOp{Column: column, Op: filterOp, Value: value, Negated: negated}, nil

	case "is":
		if value != "null" && value != "not.null" {
			return FilterOp{}, fmt.Errorf("invalid 'is' filter for %s: must be 'is.null' or 'is.not.null'", column)
		}
		return FilterOp{Column: column, Op: "is", Value: value, Negated: negated}, nil

	case "in":
		if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
			return FilterOp{}, fmt.Errorf("invalid 'in' filter syntax for %s: must be in.(a,b,c)", column)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
		parts := strings.Split(inner, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return FilterOp{Column: column, Op: "in", Values: parts, Negated: negated}, nil

	case "cs":
		if !strings.HasPrefix(value, "{") || !strings.HasSuffix(value, "}") {
			return FilterOp{}, fmt.Errorf("invalid 'cs' filter syntax for %s: must be cs.{a,b}", column)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "{"), "}")
		parts := strings.Split(inner, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return FilterOp{Column: column, Op: "cs", Values: parts, Negated: negated}, nil

	default:
		if strings.HasPrefix(filterOp, "fts(") && strings.HasSuffix(filterOp, ")") {
			lang := strings.TrimSuffix(strings.TrimPrefix(filterOp, "fts("), ")")
			if !common.IsValidIdentifier(lang) {
				return FilterOp{}, fmt.Errorf("invalid fts language: %s", lang)
			}
			return FilterOp{Column: column, Op: "fts", Value: value, Extra: lang, Negated: negated}, nil
		}
		return FilterOp{}, fmt.Errorf("unknown filter clause: %s", filterOp)
	}
}

func parseOrder(raw string) ([]OrderBy, error) {
	var orders []OrderBy
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		chunks := strings.Split(part, ".")
		column := chunks[0]
		if !common.IsValidIdentifier(column) {
			return nil, fmt.Errorf("invalid order column: %s", column)
		}

		desc := false
		nullsLast := false
		for _, modifier := range chunks[1:] {
			switch strings.ToLower(modifier) {
			case "desc":
				desc = true
			case "asc":
				desc = false
			case "nullslast":
				nullsLast = true
			}
		}

		orders = append(orders, OrderBy{
			Column:    column,
			Desc:      desc,
			NullsLast: nullsLast,
		})
	}
	return orders, nil
}

func parseSelect(raw string) ([]string, []EmbeddedField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return []string{"*"}, nil, nil
	}

	var fields []string
	var embedded []EmbeddedField

	tokens := tokenizeSelect(raw)
	for _, token := range tokens {
		if strings.Contains(token, "(") && strings.HasSuffix(token, ")") {
			openParen := strings.Index(token, "(")
			relation := token[:openParen]
			if !common.IsValidIdentifier(relation) {
				return nil, nil, fmt.Errorf("invalid embedded relation identifier: %s", relation)
			}
			inner := token[openParen+1 : len(token)-1]
			subFields, subEmbed, err := parseSelect(inner)
			if err != nil {
				return nil, nil, err
			}
			embedded = append(embedded, EmbeddedField{
				Relation: relation,
				Fields:   subFields,
				Children: subEmbed,
			})
		} else {
			if token != "*" && !common.IsValidIdentifier(token) {
				return nil, nil, fmt.Errorf("invalid field identifier: %s", token)
			}
			fields = append(fields, token)
		}
	}

	return fields, embedded, nil
}

func tokenizeSelect(selectExpression string) []string {
	var results []string
	var current strings.Builder
	depth := 0

	for _, currentChar := range selectExpression {
		switch currentChar {
		case '(':
			depth++
			current.WriteRune(currentChar)
		case ')':
			depth--
			current.WriteRune(currentChar)
		case ',':
			if depth == 0 {
				if current.Len() > 0 {
					results = append(results, strings.TrimSpace(current.String()))
					current.Reset()
				}
			} else {
				current.WriteRune(currentChar)
			}
		default:
			current.WriteRune(currentChar)
		}
	}

	if current.Len() > 0 {
		results = append(results, strings.TrimSpace(current.String()))
	}

	return results
}
