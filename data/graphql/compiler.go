// Package graphql provides a single-shot GraphQL engine with schema introspection and caching.
package graphql

import (
	"fmt"
	"sort"
	"strings"

	"layr.sh/data/common"
)

// SQLQuery represents the compiled single-shot SQL and parameter values.
type SQLQuery struct {
	SQL  string
	Args []any
}

// Compiler converts GraphQL OperationNode ASTs into single-shot SQL expressions.
type Compiler struct {
	schemaIntrospector *SchemaIntrospector
	defaultSchema      string
	allowedSchemas     []string
	excludedTables     []string
}

// NewCompiler initializes a GraphQL AST to SQL compiler.
func NewCompiler(schemaIntrospector *SchemaIntrospector, defaultSchema string) *Compiler {
	if defaultSchema == "" {
		defaultSchema = "public"
	}
	return &Compiler{
		schemaIntrospector: schemaIntrospector,
		defaultSchema:      defaultSchema,
	}
}

// SetAllowedSchemas configures which schemas may be queried through GraphQL.
func (compiler *Compiler) SetAllowedSchemas(schemas []string) {
	compiler.allowedSchemas = schemas
}

// SetExcludedTables configures which tables are restricted from GraphQL operations.
func (compiler *Compiler) SetExcludedTables(excludedTables []string) {
	compiler.excludedTables = excludedTables
}

func (compiler *Compiler) isSchemaAllowed(schema string) bool {
	if len(compiler.allowedSchemas) == 0 {
		return true
	}
	for _, allowed := range compiler.allowedSchemas {
		if allowed == schema {
			return true
		}
	}
	return false
}

func (compiler *Compiler) isTableExcluded(schema, table string) bool {
	for _, excluded := range compiler.excludedTables {
		if excluded == table || excluded == fmt.Sprintf("%s.%s", schema, table) {
			return true
		}
	}
	return false
}

// Compile compiles the entire GraphQL AST into a single-shot JSON SQL statement.
func (compiler *Compiler) Compile(operationNode *OperationNode) (SQLQuery, error) {
	if len(operationNode.SelectionSet) == 0 {
		return SQLQuery{}, fmt.Errorf("empty selection set in GraphQL operation")
	}

	var args []any
	argIndex := 1

	if operationNode.Type == MutationOperationType {
		return compiler.compileMutations(operationNode.SelectionSet)
	}

	var fieldPairs []string
	for _, rootField := range operationNode.SelectionSet {
		fieldAlias := rootField.Name
		if rootField.Alias != "" {
			fieldAlias = rootField.Alias
		}

		tableQuery, subArgs, nextArgIndex, err := compiler.compileRootQuery(rootField, argIndex)
		if err != nil {
			return SQLQuery{}, err
		}
		args = append(args, subArgs...)
		argIndex = nextArgIndex

		fieldPairs = append(fieldPairs, fmt.Sprintf("'%s', (%s)", fieldAlias, tableQuery))
	}

	finalSQL := fmt.Sprintf("SELECT json_build_object(%s)", strings.Join(fieldPairs, ", "))
	return SQLQuery{
		SQL:  finalSQL,
		Args: args,
	}, nil
}

func (compiler *Compiler) compileRootQuery(fieldNode FieldNode, startIndex int) (string, []any, int, error) {
	table := fieldNode.Name
	schema := compiler.defaultSchema

	// Allow schema qualification if field is schema_table
	if strings.Contains(table, "_") {
		for i := 1; i < len(table); i++ {
			if table[i] == '_' {
				s := table[:i]
				t := table[i+1:]
				if _, ok := compiler.schemaIntrospector.GetTable(s, t); ok {
					schema = s
					table = t
					break
				}
			}
		}
	}

	if !compiler.isSchemaAllowed(schema) {
		return "", nil, startIndex, fmt.Errorf("schema %q is not exposed for GraphQL operations", schema)
	}
	if compiler.isTableExcluded(schema, table) {
		return "", nil, startIndex, fmt.Errorf("table %q is excluded from GraphQL operations", table)
	}

	alias := "t0"
	var args []any
	argIndex := startIndex

	// Validate root field name and alias
	if !common.IsValidIdentifier(table) {
		return "", nil, argIndex, fmt.Errorf("invalid table identifier %q", table)
	}
	if fieldNode.Alias != "" && !common.IsValidIdentifier(fieldNode.Alias) {
		return "", nil, argIndex, fmt.Errorf("invalid field alias %q", fieldNode.Alias)
	}

	// Sub-selections
	var selectPairs []string
	for _, subField := range fieldNode.SelectionSet {
		subAlias := subField.Name
		if subField.Alias != "" {
			subAlias = subField.Alias
		}

		if !common.IsValidIdentifier(subAlias) {
			return "", nil, argIndex, fmt.Errorf("invalid selection alias %q", subAlias)
		}

		if len(subField.SelectionSet) > 0 {
			// Nested relation
			relQuery, relArgs, nextArgIndex, err := compiler.compileNestedRelation(schema, table, alias, subField, argIndex, 1)
			if err != nil {
				return "", nil, argIndex, err
			}
			args = append(args, relArgs...)
			argIndex = nextArgIndex
			selectPairs = append(selectPairs, fmt.Sprintf("'%s', (%s)", subAlias, relQuery))
		} else {
			// Scalar column
			if !common.IsValidIdentifier(subField.Name) {
				return "", nil, argIndex, fmt.Errorf("invalid column identifier %q", subField.Name)
			}
			selectPairs = append(selectPairs, fmt.Sprintf("'%s', %s.\"%s\"", subAlias, alias, subField.Name))
		}
	}

	if len(selectPairs) == 0 {
		if tableInfo, ok := compiler.schemaIntrospector.GetTable(schema, table); ok && len(tableInfo.Columns) > 0 {
			var columnPairs []string
			for columnName := range tableInfo.Columns {
				columnPairs = append(columnPairs, fmt.Sprintf("'%s', %s.\"%s\"", columnName, alias, columnName))
			}
			sort.Strings(columnPairs)
			selectPairs = append(selectPairs, columnPairs...)
		} else {
			selectPairs = append(selectPairs, fmt.Sprintf("'%s', to_jsonb(%s)", table, alias))
		}
	}

	whereClause, whereArgs, nextArgIndex, err := compiler.compileWhere(alias, fieldNode.Arguments["where"], argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}
	args = append(args, whereArgs...)
	argIndex = nextArgIndex

	orderByClause, err := compiler.compileOrderBy(alias, fieldNode.Arguments["order_by"])
	if err != nil {
		return "", nil, argIndex, err
	}

	jsonRow := fmt.Sprintf("json_build_object(%s)", strings.Join(selectPairs, ", "))
	subQuery := fmt.Sprintf("SELECT COALESCE(json_agg(%s), '[]'::json) FROM \"%s\".\"%s\" %s", jsonRow, schema, table, alias)
	if whereClause != "" {
		subQuery += " WHERE " + whereClause
	}
	if orderByClause != "" {
		subQuery += orderByClause
	}

	// Limit and Offset (strictly type-asserted integers)
	if limitArg, ok := fieldNode.Arguments["limit"]; ok {
		switch typedLimit := limitArg.(type) {
		case int:
			subQuery += fmt.Sprintf(" LIMIT %d", typedLimit)
		case int64:
			subQuery += fmt.Sprintf(" LIMIT %d", typedLimit)
		case float64:
			subQuery += fmt.Sprintf(" LIMIT %d", int64(typedLimit))
		default:
			return "", nil, argIndex, fmt.Errorf("limit must be an integer")
		}
	}
	if offsetArg, ok := fieldNode.Arguments["offset"]; ok {
		switch typedOffset := offsetArg.(type) {
		case int:
			subQuery += fmt.Sprintf(" OFFSET %d", typedOffset)
		case int64:
			subQuery += fmt.Sprintf(" OFFSET %d", typedOffset)
		case float64:
			subQuery += fmt.Sprintf(" OFFSET %d", int64(typedOffset))
		default:
			return "", nil, argIndex, fmt.Errorf("offset must be an integer")
		}
	}

	return subQuery, args, argIndex, nil
}

func (compiler *Compiler) compileNestedRelation(schema, parentTable, parentAlias string, fieldNode FieldNode, startIndex, depth int) (string, []any, int, error) {
	relTable := fieldNode.Name
	if !common.IsValidIdentifier(relTable) {
		return "", nil, startIndex, fmt.Errorf("invalid relation table %q", relTable)
	}

	relationAlias := fmt.Sprintf("t%d", depth)
	var args []any
	argIndex := startIndex

	var selectPairs []string
	for _, subField := range fieldNode.SelectionSet {
		subAlias := subField.Name
		if subField.Alias != "" {
			subAlias = subField.Alias
		}

		if !common.IsValidIdentifier(subAlias) {
			return "", nil, argIndex, fmt.Errorf("invalid selection alias %q", subAlias)
		}

		if len(subField.SelectionSet) > 0 {
			nestedQuery, nestedArgs, nextArgIndex, err := compiler.compileNestedRelation(schema, relTable, relationAlias, subField, argIndex, depth+1)
			if err != nil {
				return "", nil, argIndex, err
			}
			args = append(args, nestedArgs...)
			argIndex = nextArgIndex
			selectPairs = append(selectPairs, fmt.Sprintf("'%s', (%s)", subAlias, nestedQuery))
		} else {
			if !common.IsValidIdentifier(subField.Name) {
				return "", nil, argIndex, fmt.Errorf("invalid column identifier %q", subField.Name)
			}
			selectPairs = append(selectPairs, fmt.Sprintf("'%s', %s.\"%s\"", subAlias, relationAlias, subField.Name))
		}
	}

	jsonRow := fmt.Sprintf("json_build_object(%s)", strings.Join(selectPairs, ", "))

	// Foreign key join predicate (supports bidirectional relations)
	parentPrimaryKey := "id"
	foreignKeyColumn := fmt.Sprintf("%s_id", strings.TrimSuffix(parentTable, "s"))
	actualRelTable := relTable
	actualRelSchema := schema

	parentTableInfo, hasParentInfo := compiler.schemaIntrospector.GetTable(schema, parentTable)
	if hasParentInfo && parentTableInfo.PrimaryKey != "" {
		parentPrimaryKey = parentTableInfo.PrimaryKey
	}

	relationTableInfo, hasRelInfo := compiler.schemaIntrospector.GetTable(schema, relTable)
	if !hasRelInfo {
		if strings.HasSuffix(relTable, "s") {
			relationTableInfo, hasRelInfo = compiler.schemaIntrospector.GetTable(schema, strings.TrimSuffix(relTable, "s"))
			if hasRelInfo {
				actualRelTable = strings.TrimSuffix(relTable, "s")
			}
		} else {
			relationTableInfo, hasRelInfo = compiler.schemaIntrospector.GetTable(schema, relTable+"s")
			if hasRelInfo {
				actualRelTable = relTable + "s"
			}
		}
	}

	if hasRelInfo {
		if relationInfo, ok := relationTableInfo.ForeignKeys[parentTable]; ok {
			foreignKeyColumn = relationInfo.LocalColumn
			parentPrimaryKey = relationInfo.TargetColumn
			if relationInfo.ForeignTable != "" && !relationInfo.IsArray {
				actualRelTable = relationInfo.ForeignTable
			}
		}
	}
	if hasParentInfo {
		if parentRelationInfo, ok := parentTableInfo.ForeignKeys[relTable]; ok {
			foreignKeyColumn = parentRelationInfo.TargetColumn
			parentPrimaryKey = parentRelationInfo.LocalColumn
			if parentRelationInfo.ForeignTable != "" {
				actualRelTable = parentRelationInfo.ForeignTable
			}
			if parentRelationInfo.ForeignSchema != "" {
				actualRelSchema = parentRelationInfo.ForeignSchema
			}
		}
	}
	joinCondition := fmt.Sprintf("%s.\"%s\" = %s.\"%s\"", relationAlias, foreignKeyColumn, parentAlias, parentPrimaryKey)

	whereClause, whereArgs, nextArgIndex, err := compiler.compileWhere(relationAlias, fieldNode.Arguments["where"], argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}
	args = append(args, whereArgs...)
	argIndex = nextArgIndex

	orderByClause, err := compiler.compileOrderBy(relationAlias, fieldNode.Arguments["order_by"])
	if err != nil {
		return "", nil, argIndex, err
	}

	fullWhere := joinCondition
	if whereClause != "" {
		fullWhere += " AND " + whereClause
	}

	query := fmt.Sprintf("SELECT COALESCE(json_agg(%s), '[]'::json) FROM \"%s\".\"%s\" %s WHERE %s",
		jsonRow, actualRelSchema, actualRelTable, relationAlias, fullWhere)
	if orderByClause != "" {
		query += orderByClause
	}
	if limitArg, ok := fieldNode.Arguments["limit"]; ok {
		switch typedLimit := limitArg.(type) {
		case int:
			query += fmt.Sprintf(" LIMIT %d", typedLimit)
		case int64:
			query += fmt.Sprintf(" LIMIT %d", typedLimit)
		case float64:
			query += fmt.Sprintf(" LIMIT %d", int64(typedLimit))
		default:
			return "", nil, argIndex, fmt.Errorf("limit must be an integer")
		}
	}
	if offsetArg, ok := fieldNode.Arguments["offset"]; ok {
		switch typedOffset := offsetArg.(type) {
		case int:
			query += fmt.Sprintf(" OFFSET %d", typedOffset)
		case int64:
			query += fmt.Sprintf(" OFFSET %d", typedOffset)
		case float64:
			query += fmt.Sprintf(" OFFSET %d", int64(typedOffset))
		default:
			return "", nil, argIndex, fmt.Errorf("offset must be an integer")
		}
	}

	return query, args, argIndex, nil
}

func (compiler *Compiler) compileOrderBy(tableAlias string, orderBy any) (string, error) {
	if orderBy == nil {
		return "", nil
	}
	orderMap, ok := orderBy.(map[string]any)
	if !ok {
		return "", fmt.Errorf("order_by argument must be an object")
	}
	if len(orderMap) == 0 {
		return "", nil
	}
	var orderParts []string
	for columnName, dir := range orderMap {
		if !common.IsValidIdentifier(columnName) {
			return "", fmt.Errorf("invalid column identifier %q in order_by", columnName)
		}
		direction := strings.ToUpper(fmt.Sprintf("%v", dir))
		if direction != "ASC" && direction != "DESC" {
			direction = "ASC"
		}
		orderParts = append(orderParts, fmt.Sprintf("%s.\"%s\" %s", tableAlias, columnName, direction))
	}
	sort.Strings(orderParts)
	return " ORDER BY " + strings.Join(orderParts, ", "), nil
}

func (compiler *Compiler) compileWhere(tableAlias string, where any, startIndex int) (string, []any, int, error) {
	if where == nil {
		return "", nil, startIndex, nil
	}

	whereMap, ok := where.(map[string]any)
	if !ok {
		return "", nil, startIndex, fmt.Errorf("where argument must be an object")
	}
	if len(whereMap) == 0 {
		return "", nil, startIndex, nil
	}

	var clauses []string
	var args []any
	argIndex := startIndex

	for columnName, condition := range whereMap {
		if !common.IsValidIdentifier(columnName) {
			return "", nil, startIndex, fmt.Errorf("invalid column identifier %q in where clause", columnName)
		}
		conditionMap, isMap := condition.(map[string]any)
		if isMap {
			for filterOp, conditionValue := range conditionMap {
				switch filterOp {
				case "_eq":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" = $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_neq":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" != $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_gt":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" > $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_gte":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" >= $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_lt":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" < $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_lte":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" <= $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_like":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" LIKE $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_ilike":
					clauses = append(clauses, fmt.Sprintf("%s.\"%s\" ILIKE $%d", tableAlias, columnName, argIndex))
					args = append(args, conditionValue)
					argIndex++
				case "_in":
					if inArray, isArr := conditionValue.([]any); isArr {
						if len(inArray) == 0 {
							clauses = append(clauses, "FALSE")
						} else {
							var place []string
							for range inArray {
								place = append(place, fmt.Sprintf("$%d", argIndex))
								argIndex++
							}
							args = append(args, inArray...)
							clauses = append(clauses, fmt.Sprintf("%s.\"%s\" IN (%s)", tableAlias, columnName, strings.Join(place, ", ")))
						}
					}
				case "_is_null":
					if isNull, ok := conditionValue.(bool); ok && isNull {
						clauses = append(clauses, fmt.Sprintf("%s.\"%s\" IS NULL", tableAlias, columnName))
					} else {
						clauses = append(clauses, fmt.Sprintf("%s.\"%s\" IS NOT NULL", tableAlias, columnName))
					}
				}
			}
		} else {
			// Direct equality
			clauses = append(clauses, fmt.Sprintf("%s.\"%s\" = $%d", tableAlias, columnName, argIndex))
			args = append(args, condition)
			argIndex++
		}
	}

	return strings.Join(clauses, " AND "), args, argIndex, nil
}

func (compiler *Compiler) compileMutations(fields []FieldNode) (SQLQuery, error) {
	var commonTableExpressions []string
	var selectPairs []string
	var args []any
	argIndex := 1

	for mutationIndex, mutationField := range fields {
		cteName := fmt.Sprintf("m%d", mutationIndex)
		name := mutationField.Name

		if strings.HasPrefix(name, "insert_") {
			table := strings.TrimPrefix(name, "insert_")
			targetSchema := compiler.defaultSchema
			if strings.Contains(table, "_") {
				for i := 1; i < len(table); i++ {
					if table[i] == '_' {
						s := table[:i]
						t := table[i+1:]
						if _, ok := compiler.schemaIntrospector.GetTable(s, t); ok {
							targetSchema = s
							table = t
							break
						}
					}
				}
			}
			if !compiler.isSchemaAllowed(targetSchema) {
				return SQLQuery{}, fmt.Errorf("schema %q is not exposed for GraphQL operations", targetSchema)
			}
			if compiler.isTableExcluded(targetSchema, table) {
				return SQLQuery{}, fmt.Errorf("table %q is excluded from GraphQL operations", table)
			}
			if !common.IsValidIdentifier(table) {
				return SQLQuery{}, fmt.Errorf("invalid table identifier %q in insert mutation", table)
			}
			objects, ok := mutationField.Arguments["objects"].([]any)
			if !ok || len(objects) == 0 {
				return SQLQuery{}, fmt.Errorf("insert mutation requires 'objects' argument")
			}

			recordMap, ok := objects[0].(map[string]any)
			if !ok {
				return SQLQuery{}, fmt.Errorf("objects elements must be maps")
			}

			var columns []string
			for columnName := range recordMap {
				if !common.IsValidIdentifier(columnName) {
					return SQLQuery{}, fmt.Errorf("invalid column identifier %q in insert mutation", columnName)
				}
				columns = append(columns, columnName)
			}
			sort.Strings(columns)
			var quotedColumns []string
			for _, columnName := range columns {
				quotedColumns = append(quotedColumns, fmt.Sprintf(`"%s"`, columnName))
			}

			var valueRows []string
			for _, objectRow := range objects {
				objectMap := objectRow.(map[string]any)
				var placeholders []string
				for _, column := range columns {
					placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
					args = append(args, objectMap[column])
					argIndex++
				}
				valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(placeholders, ", ")))
			}

			commonTableExpression := fmt.Sprintf(`%s AS (
				INSERT INTO "%s"."%s" (%s) VALUES %s RETURNING *
			)`, cteName, targetSchema, table, strings.Join(quotedColumns, ", "), strings.Join(valueRows, ", "))
			commonTableExpressions = append(commonTableExpressions, commonTableExpression)
		} else if strings.HasPrefix(name, "update_") {
			table := strings.TrimPrefix(name, "update_")
			targetSchema := compiler.defaultSchema
			if strings.Contains(table, "_") {
				for i := 1; i < len(table); i++ {
					if table[i] == '_' {
						s := table[:i]
						t := table[i+1:]
						if _, ok := compiler.schemaIntrospector.GetTable(s, t); ok {
							targetSchema = s
							table = t
							break
						}
					}
				}
			}
			if !compiler.isSchemaAllowed(targetSchema) {
				return SQLQuery{}, fmt.Errorf("schema %q is not exposed for GraphQL operations", targetSchema)
			}
			if compiler.isTableExcluded(targetSchema, table) {
				return SQLQuery{}, fmt.Errorf("table %q is excluded from GraphQL operations", table)
			}
			if !common.IsValidIdentifier(table) {
				return SQLQuery{}, fmt.Errorf("invalid table identifier %q in update mutation", table)
			}
			updateMap, ok := mutationField.Arguments["_set"].(map[string]any)
			if !ok || len(updateMap) == 0 {
				return SQLQuery{}, fmt.Errorf("update mutation requires '_set' argument")
			}

			var setParts []string
			for columnName, columnValue := range updateMap {
				if !common.IsValidIdentifier(columnName) {
					return SQLQuery{}, fmt.Errorf("invalid column identifier %q in update mutation", columnName)
				}
				setParts = append(setParts, fmt.Sprintf(`"%s" = $%d`, columnName, argIndex))
				args = append(args, columnValue)
				argIndex++
			}

			whereArg, hasWhere := mutationField.Arguments["where"]
			if !hasWhere || whereArg == nil {
				return SQLQuery{}, fmt.Errorf("update mutation requires a non-empty 'where' argument for safety")
			}
			whereClause, whereArgs, nextArgIndex, err := compiler.compileWhere(table, whereArg, argIndex)
			if err != nil {
				return SQLQuery{}, err
			}
			if whereClause == "" {
				return SQLQuery{}, fmt.Errorf("update mutation requires a non-empty 'where' argument for safety")
			}
			args = append(args, whereArgs...)
			argIndex = nextArgIndex

			commonTableExpression := fmt.Sprintf(`%s AS (
				UPDATE "%s"."%s" SET %s WHERE %s RETURNING *
			)`, cteName, targetSchema, table, strings.Join(setParts, ", "), whereClause)
			commonTableExpressions = append(commonTableExpressions, commonTableExpression)
		} else if strings.HasPrefix(name, "delete_") {
			table := strings.TrimPrefix(name, "delete_")
			targetSchema := compiler.defaultSchema
			if strings.Contains(table, "_") {
				for i := 1; i < len(table); i++ {
					if table[i] == '_' {
						s := table[:i]
						t := table[i+1:]
						if _, ok := compiler.schemaIntrospector.GetTable(s, t); ok {
							targetSchema = s
							table = t
							break
						}
					}
				}
			}
			if !compiler.isSchemaAllowed(targetSchema) {
				return SQLQuery{}, fmt.Errorf("schema %q is not exposed for GraphQL operations", targetSchema)
			}
			if compiler.isTableExcluded(targetSchema, table) {
				return SQLQuery{}, fmt.Errorf("table %q is excluded from GraphQL operations", table)
			}
			if !common.IsValidIdentifier(table) {
				return SQLQuery{}, fmt.Errorf("invalid table identifier %q in delete mutation", table)
			}

			whereArg, hasWhere := mutationField.Arguments["where"]
			if !hasWhere || whereArg == nil {
				return SQLQuery{}, fmt.Errorf("delete mutation requires a non-empty 'where' argument for safety")
			}
			whereClause, whereArgs, nextArgIndex, err := compiler.compileWhere(table, whereArg, argIndex)
			if err != nil {
				return SQLQuery{}, err
			}
			if whereClause == "" {
				return SQLQuery{}, fmt.Errorf("delete mutation requires a non-empty 'where' argument for safety")
			}
			args = append(args, whereArgs...)
			argIndex = nextArgIndex

			commonTableExpression := fmt.Sprintf(`%s AS (
				DELETE FROM "%s"."%s" WHERE %s RETURNING *
			)`, cteName, targetSchema, table, whereClause)
			commonTableExpressions = append(commonTableExpressions, commonTableExpression)
		} else {
			return SQLQuery{}, fmt.Errorf("unsupported mutation: %s", name)
		}

		// Return only selected fields if selection set is provided
		var returnSelectPairs []string
		for _, subField := range mutationField.SelectionSet {
			subAlias := subField.Name
			if subField.Alias != "" {
				subAlias = subField.Alias
			}
			if common.IsValidIdentifier(subField.Name) && common.IsValidIdentifier(subAlias) {
				returnSelectPairs = append(returnSelectPairs, fmt.Sprintf("'%s', %s.\"%s\"", subAlias, cteName, subField.Name))
			}
		}
		var jsonAggExpr string
		if len(returnSelectPairs) > 0 {
			jsonAggExpr = fmt.Sprintf("json_build_object(%s)", strings.Join(returnSelectPairs, ", "))
		} else {
			jsonAggExpr = fmt.Sprintf("to_json(%s)", cteName)
		}

		selectPairs = append(selectPairs, fmt.Sprintf("'%s', (SELECT COALESCE(json_agg(%s), '[]'::json) FROM %s)", name, jsonAggExpr, cteName))
	}

	finalSQL := fmt.Sprintf("WITH %s SELECT json_build_object(%s)", strings.Join(commonTableExpressions, ", "), strings.Join(selectPairs, ", "))
	return SQLQuery{
		SQL:  finalSQL,
		Args: args,
	}, nil
}
