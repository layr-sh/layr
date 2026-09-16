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

// RelationForeignKey defines relation details between tables.
type RelationForeignKey struct {
	FromColumn string
	ToTable    string
	ToColumn   string
}

// QueryBuilder constructs parameterized SQL queries.
type QueryBuilder struct {
	schema string
	table  string
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

	// Handle embedded relations
	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	for _, embedded := range queryParams.Embedded {
		var jsonAggExpression string
		if len(embedded.Fields) > 0 && (len(embedded.Fields) != 1 || embedded.Fields[0] != "*") {
			var jsonBuildObjectPairs []string
			for _, fieldName := range embedded.Fields {
				if common.IsValidIdentifier(fieldName) {
					jsonBuildObjectPairs = append(jsonBuildObjectPairs, fmt.Sprintf("'%s', r.\"%s\"", fieldName, fieldName))
				}
			}
			if len(jsonBuildObjectPairs) > 0 {
				jsonAggExpression = fmt.Sprintf("json_build_object(%s)", strings.Join(jsonBuildObjectPairs, ", "))
			} else {
				jsonAggExpression = "to_json(r)"
			}
		} else {
			jsonAggExpression = "to_json(r)"
		}

		relationForeignKey, ok := tableMetadata.ForeignKeys[embedded.Relation]
		if !ok {
			// Fallback convention: relation is foreign table with <table_singular>_id = table.id
			// or relation_id on current table
			subSQL := fmt.Sprintf(`(
				SELECT COALESCE(json_agg(%s), '[]'::json)
				FROM "%s"."%s" r
				WHERE r."%s_id" = "%s"."%s"
			) AS "%s"`, jsonAggExpression, queryBuilder.schema, embedded.Relation, strings.TrimSuffix(queryBuilder.table, "s"), queryBuilder.table, primaryKey, embedded.Relation)
			selectColumns = append(selectColumns, subSQL)
		} else {
			subSQL := fmt.Sprintf(`(
				SELECT COALESCE(json_agg(%s), '[]'::json)
				FROM "%s"."%s" r
				WHERE r."%s" = "%s"."%s"
			) AS "%s"`, jsonAggExpression, queryBuilder.schema, relationForeignKey.ToTable, relationForeignKey.ToColumn, queryBuilder.table, relationForeignKey.FromColumn, embedded.Relation)
			selectColumns = append(selectColumns, subSQL)
		}
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
