package rest

import (
	"fmt"
	"strings"

	"layr.sh/data/common"
)

// SQLStatement holds the parameterized query and argument list.
type SQLStatement struct {
	SQL  string
	Args []any
}

// TableMetadata contains basic table information for relational embedding.
type TableMetadata struct {
	Schema      string
	Table       string
	PrimaryKey  string
	Columns     []string
	ForeignKeys map[string]RelationForeignKey // relationName -> FK details
}

// RelationForeignKey defines relationForeignKey details between tables.
type RelationForeignKey struct {
	FromColumn string
	ToTable    string
	ToSchema   string
	ToColumn   string
}

// QueryBuilder constructs parameterized SQL queries.
type QueryBuilder struct {
	schema           string
	table            string
	metadataResolver func(string, string) (TableMetadata, bool)
}

// NewQueryBuilder initializes a query builder for a specific schema and table.
func NewQueryBuilder(schema, table string) *QueryBuilder {
	if schema == "" {
		schema = "public"
	}
	return &QueryBuilder{
		schema: schema,
		table:  table,
	}
}

// SetMetadataResolver supplies exposed relation metadata for nested selects.
func (queryBuilder *QueryBuilder) SetMetadataResolver(resolver func(string, string) (TableMetadata, bool)) {
	queryBuilder.metadataResolver = resolver
}

func (queryBuilder *QueryBuilder) buildEmbedded(embeddedField EmbeddedField, tableMetadata TableMetadata, schema, table, parentAlias string, depth int) (string, error) {
	if depth > 16 || !common.IsValidIdentifier(embeddedField.Relation) {
		return "", fmt.Errorf("invalid or excessively nested relation")
	}
	targetSchema, targetTable := schema, embeddedField.Relation
	sourceColumn := tableMetadata.PrimaryKey
	if sourceColumn == "" {
		sourceColumn = "id"
	}
	targetColumn := strings.TrimSuffix(table, "s") + "_id"
	if relationForeignKey, exists := tableMetadata.ForeignKeys[embeddedField.Relation]; exists {
		targetTable = relationForeignKey.ToTable
		targetColumn = relationForeignKey.ToColumn
		sourceColumn = relationForeignKey.FromColumn
		if relationForeignKey.ToSchema != "" {
			targetSchema = relationForeignKey.ToSchema
		}
	}
	if _, exists := tableMetadata.ForeignKeys[embeddedField.Relation]; !exists && queryBuilder.metadataResolver != nil {
		return "", fmt.Errorf("no unambiguous relationship named %q", embeddedField.Relation)
	}
	if common.IsProtectedSchema(targetSchema) || !common.IsValidIdentifier(targetSchema) || !common.IsValidIdentifier(targetTable) || !common.IsValidIdentifier(sourceColumn) || !common.IsValidIdentifier(targetColumn) {
		return "", fmt.Errorf("invalid relation target")
	}
	targetTableMetadata := TableMetadata{Schema: targetSchema, Table: targetTable}
	if queryBuilder.metadataResolver != nil {
		var allowed bool
		targetTableMetadata, allowed = queryBuilder.metadataResolver(targetSchema, targetTable)
		if !allowed {
			return "", fmt.Errorf("relation %s.%s is not exposed", targetSchema, targetTable)
		}
	}
	alias := fmt.Sprintf("relation_%d", depth)
	var columns []string
	if len(embeddedField.Fields) == 0 {
		columns = append(columns, alias+".*")
	}
	for _, name := range embeddedField.Fields {
		if name == "*" {
			columns = append(columns, alias+".*")
			continue
		}
		if !common.IsValidIdentifier(name) {
			return "", fmt.Errorf("invalid relation column")
		}
		columns = append(columns, fmt.Sprintf(`%s."%s"`, alias, name))
	}
	for _, child := range embeddedField.Children {
		expression, err := queryBuilder.buildEmbedded(child, targetTableMetadata, targetSchema, targetTable, alias, depth+1)
		if err != nil {
			return "", err
		}
		columns = append(columns, expression+fmt.Sprintf(` AS "%s"`, child.Relation))
	}
	return fmt.Sprintf(`(SELECT COALESCE(json_agg(embedded_row), '[]'::json) FROM (SELECT %s FROM "%s"."%s" %s WHERE %s."%s" = "%s"."%s") embedded_row)`, strings.Join(columns, ", "), targetSchema, targetTable, alias, alias, targetColumn, parentAlias, sourceColumn), nil
}

// BuildSelect builds a parameterized SELECT query.
func (queryBuilder *QueryBuilder) BuildSelect(queryParams *QueryParams, tableMetadata TableMetadata) (SQLStatement, error) {
	var args []any
	argIndex := 1

	// Select fields
	var selectColumns []string
	if len(queryParams.Fields) == 0 || (len(queryParams.Fields) == 1 && queryParams.Fields[0] == "*") {
		selectColumns = append(selectColumns, fmt.Sprintf(`"%s".*`, queryBuilder.table))
	} else {
		for _, fieldName := range queryParams.Fields {
			if fieldName == "*" {
				selectColumns = append(selectColumns, fmt.Sprintf(`"%s".*`, queryBuilder.table))
			} else {
				selectColumns = append(selectColumns, fmt.Sprintf(`"%s"."%s"`, queryBuilder.table, fieldName))
			}
		}
	}

	for _, embedded := range queryParams.Embedded {
		expression, err := queryBuilder.buildEmbedded(embedded, tableMetadata, queryBuilder.schema, queryBuilder.table, queryBuilder.table, 1)
		if err != nil {
			return SQLStatement{}, err
		}
		selectColumns = append(selectColumns, expression+fmt.Sprintf(` AS "%s"`, embedded.Relation))
	}

	whereClause, whereArgs, err := queryBuilder.buildWhere(queryParams.Filters, argIndex)
	if err != nil {
		return SQLStatement{}, err
	}
	args = append(args, whereArgs...)

	query := fmt.Sprintf(`SELECT %s FROM "%s"."%s"`, strings.Join(selectColumns, ", "), queryBuilder.schema, queryBuilder.table)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	// Order by
	if len(queryParams.Orders) > 0 {
		var orderParts []string
		for _, orderOp := range queryParams.Orders {
			direction := "ASC"
			if orderOp.Desc {
				direction = "DESC"
			}
			part := fmt.Sprintf(`"%s"."%s" %s`, queryBuilder.table, orderOp.Column, direction)
			if orderOp.NullsLast {
				part += " NULLS LAST"
			}
			orderParts = append(orderParts, part)
		}
		query += " ORDER BY " + strings.Join(orderParts, ", ")
	}

	// Limit and Offset
	if queryParams.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", queryParams.Limit)
	}
	if queryParams.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", queryParams.Offset)
	}

	return SQLStatement{
		SQL:  query,
		Args: args,
	}, nil
}

// BuildCount builds a parameterized COUNT query.
func (queryBuilder *QueryBuilder) BuildCount(queryParams *QueryParams) (SQLStatement, error) {
	whereClause, whereArgs, err := queryBuilder.buildWhere(queryParams.Filters, 1)
	if err != nil {
		return SQLStatement{}, err
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"."%s"`, queryBuilder.schema, queryBuilder.table)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return SQLStatement{
		SQL:  query,
		Args: whereArgs,
	}, nil
}

// BuildInsert builds a parameterized INSERT query for single or multiple records.
func (queryBuilder *QueryBuilder) BuildInsert(records []map[string]any, onConflict string) (SQLStatement, error) {
	if len(records) == 0 {
		return SQLStatement{}, fmt.Errorf("no records to insert")
	}

	// Collect unique columns
	columnSet := make(map[string]bool)
	var columns []string
	for _, record := range records {
		for columnName := range record {
			if !common.IsValidIdentifier(columnName) {
				return SQLStatement{}, fmt.Errorf("invalid column name in insert: %s", columnName)
			}
			if !columnSet[columnName] {
				columnSet[columnName] = true
				columns = append(columns, columnName)
			}
		}
	}

	if len(columns) == 0 {
		return SQLStatement{}, fmt.Errorf("no valid columns in insert data")
	}

	var quotedColumns []string
	for _, column := range columns {
		quotedColumns = append(quotedColumns, fmt.Sprintf(`"%s"`, column))
	}

	var valueRows []string
	var args []any
	argIndex := 1

	for _, record := range records {
		var rowPlaceholders []string
		for _, column := range columns {
			value, exists := record[column]
			if exists {
				rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("$%d", argIndex))
				args = append(args, value)
				argIndex++
			} else {
				rowPlaceholders = append(rowPlaceholders, "DEFAULT")
			}
		}
		valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(rowPlaceholders, ", ")))
	}

	query := fmt.Sprintf(`INSERT INTO "%s"."%s" (%s) VALUES %s`,
		queryBuilder.schema, queryBuilder.table, strings.Join(quotedColumns, ", "), strings.Join(valueRows, ", "))

	if onConflict != "" {
		var updateParts []string
		for _, column := range columns {
			if column != onConflict {
				updateParts = append(updateParts, fmt.Sprintf(`"%s" = EXCLUDED."%s"`, column, column))
			}
		}
		if len(updateParts) > 0 {
			query += fmt.Sprintf(` ON CONFLICT ("%s") DO UPDATE SET %s`, onConflict, strings.Join(updateParts, ", "))
		} else {
			query += fmt.Sprintf(` ON CONFLICT ("%s") DO NOTHING`, onConflict)
		}
	}

	query += " RETURNING *"

	return SQLStatement{
		SQL:  query,
		Args: args,
	}, nil
}

// BuildUpdate builds a parameterized UPDATE query.
func (queryBuilder *QueryBuilder) BuildUpdate(values map[string]any, filters []FilterOp) (SQLStatement, error) {
	if len(values) == 0 {
		return SQLStatement{}, fmt.Errorf("no values provided for update")
	}

	var args []any
	argIndex := 1
	var setParts []string

	for columnName, columnValue := range values {
		if !common.IsValidIdentifier(columnName) {
			return SQLStatement{}, fmt.Errorf("invalid column name in update: %s", columnName)
		}
		setParts = append(setParts, fmt.Sprintf(`"%s" = $%d`, columnName, argIndex))
		args = append(args, columnValue)
		argIndex++
	}

	whereClause, whereArgs, err := queryBuilder.buildWhere(filters, argIndex)
	if err != nil {
		return SQLStatement{}, err
	}
	args = append(args, whereArgs...)

	query := fmt.Sprintf(`UPDATE "%s"."%s" SET %s`, queryBuilder.schema, queryBuilder.table, strings.Join(setParts, ", "))
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " RETURNING *"

	return SQLStatement{
		SQL:  query,
		Args: args,
	}, nil
}

// BuildDelete builds a parameterized DELETE query.
func (queryBuilder *QueryBuilder) BuildDelete(filters []FilterOp) (SQLStatement, error) {
	whereClause, whereArgs, err := queryBuilder.buildWhere(filters, 1)
	if err != nil {
		return SQLStatement{}, err
	}

	query := fmt.Sprintf(`DELETE FROM "%s"."%s"`, queryBuilder.schema, queryBuilder.table)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " RETURNING *"

	return SQLStatement{
		SQL:  query,
		Args: whereArgs,
	}, nil
}

func (queryBuilder *QueryBuilder) buildWhere(filters []FilterOp, startIndex int) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	var clauses []string
	var args []any
	argIndex := startIndex

	for _, filterOp := range filters {
		if !common.IsValidIdentifier(filterOp.Column) {
			return "", nil, fmt.Errorf("invalid column in filter: %s", filterOp.Column)
		}

		var clause string
		switch filterOp.Op {
		case "eq":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" != $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" = $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "neq":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" = $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" != $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "gt":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" <= $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" > $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "gte":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" < $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" >= $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "lt":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" >= $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" < $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "lte":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" > $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" <= $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "like":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" NOT LIKE $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" LIKE $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "ilike":
			if filterOp.Negated {
				clause = fmt.Sprintf(`"%s"."%s" NOT ILIKE $%d`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" ILIKE $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		case "is":
			isNull := (filterOp.Value == "null" && !filterOp.Negated) || (filterOp.Value == "not.null" && filterOp.Negated)
			if isNull {
				clause = fmt.Sprintf(`"%s"."%s" IS NULL`, queryBuilder.table, filterOp.Column)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" IS NOT NULL`, queryBuilder.table, filterOp.Column)
			}
		case "in":
			var inPlaceholders []string
			for _, inValue := range filterOp.Values {
				inPlaceholders = append(inPlaceholders, fmt.Sprintf("$%d", argIndex))
				args = append(args, inValue)
				argIndex++
			}
			op := "IN"
			if filterOp.Negated {
				op = "NOT IN"
			}
			clause = fmt.Sprintf(`"%s"."%s" %s (%s)`, queryBuilder.table, filterOp.Column, op, strings.Join(inPlaceholders, ", "))
		case "cs":
			if filterOp.Negated {
				clause = fmt.Sprintf(`NOT ("%s"."%s" @> $%d)`, queryBuilder.table, filterOp.Column, argIndex)
			} else {
				clause = fmt.Sprintf(`"%s"."%s" @> $%d`, queryBuilder.table, filterOp.Column, argIndex)
			}
			args = append(args, filterOp.Values)
			argIndex++
		case "fts":
			if filterOp.Negated {
				clause = fmt.Sprintf(`NOT (to_tsvector('%s', "%s"."%s") @@ websearch_to_tsquery('%s', $%d))`, filterOp.Extra, queryBuilder.table, filterOp.Column, filterOp.Extra, argIndex)
			} else {
				clause = fmt.Sprintf(`to_tsvector('%s', "%s"."%s") @@ websearch_to_tsquery('%s', $%d)`, filterOp.Extra, queryBuilder.table, filterOp.Column, filterOp.Extra, argIndex)
			}
			args = append(args, filterOp.Value)
			argIndex++
		default:
			return "", nil, fmt.Errorf("unsupported filter clause: %s", filterOp.Op)
		}
		clauses = append(clauses, clause)
	}

	return strings.Join(clauses, " AND "), args, nil
}
