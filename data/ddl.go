package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"layr.sh/core"
	"layr.sh/data/common"
)

var protectedSchemas = map[string]bool{
	"system":             true,
	"auth":               true,
	"data":               true,
	"storage":            true,
	"scheduler":          true,
	"notification":       true,
	"analytics":          true,
	"console":            true,
	"information_schema": true,
	"pg_catalog":         true,
	"pg_toast":           true,
	"core":               true,
	"layr_auth":          true,
	"layr_storage":       true,
	"layr_scheduler":     true,
	"layr_notification":  true,
	"layr_analytics":     true,
	"layr_console":       true,
}

// IsProtectedSchema returns true if the schema is reserved for platform internals or PostgreSQL catalog.
func IsProtectedSchema(schema string) bool {
	return protectedSchemas[strings.ToLower(strings.TrimSpace(schema))]
}

// DDLEngine executes schema introspection and DDL commands.
type DDLEngine struct {
	db *core.DatabasePool
}

// NewDDLEngine initializes the DDL schema engine.
func NewDDLEngine(db *core.DatabasePool) *DDLEngine {
	return &DDLEngine{db: db}
}

// ListTables retrieves metadata for all user tables across the specified schemas.
func (engine *DDLEngine) ListTables(ctx context.Context, schemas []string) ([]TableSummary, error) {
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	var results []TableSummary
	for _, schema := range schemas {
		if IsProtectedSchema(schema) {
			continue
		}

		const listSQLStatement = `
			SELECT 
				t.table_name,
				COALESCE(c.reltuples::bigint, 0) AS estimated_rows,
				COALESCE(pg_total_relation_size(quote_ident(t.table_schema) || '.' || quote_ident(t.table_name)), 0) AS total_bytes
			FROM information_schema.tables t
			LEFT JOIN pg_class c ON c.relname = t.table_name AND c.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = t.table_schema)
			WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
			ORDER BY t.table_name ASC
		`
		rows, queryErr := engine.db.Query(ctx, listSQLStatement, schema)
		if queryErr != nil {
			return nil, fmt.Errorf("failed to list tables for schema %s: %w", schema, queryErr)
		}

		type tableStatsRecord struct {
			name      string
			estRows   int64
			sizeBytes int64
		}
		var tableList []tableStatsRecord
		for rows.Next() {
			var statsRecord tableStatsRecord
			if scanErr := rows.Scan(&statsRecord.name, &statsRecord.estRows, &statsRecord.sizeBytes); scanErr == nil {
				tableList = append(tableList, statsRecord)
			}
		}
		rows.Close()

		for _, statsRecord := range tableList {
			tableSummary, _ := engine.GetTable(ctx, schema, statsRecord.name)
			if tableSummary != nil {
				tableSummary.EstimatedRowCount = statsRecord.estRows
				tableSummary.SizeBytes = statsRecord.sizeBytes
				results = append(results, *tableSummary)
			}
		}
	}

	return results, nil
}

// GetTable retrieves detailed column, foreign key, and index metadata for a table.
func (engine *DDLEngine) GetTable(ctx context.Context, schema, table string) (*TableSummary, error) {
	if IsProtectedSchema(schema) {
		return nil, fmt.Errorf("schema '%s' is protected and cannot be managed via DDL", schema)
	}

	var exists bool
	const existsSQLStatement = `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = $1 AND table_name = $2 AND table_type = 'BASE TABLE'
		)
	`
	scanErr := engine.db.QueryRow(ctx, existsSQLStatement, schema, table).Scan(&exists)
	if scanErr != nil || !exists {
		return nil, fmt.Errorf("table '%s.%s' not found", schema, table)
	}

	tableSummary := &TableSummary{
		Schema:      schema,
		Name:        table,
		Columns:     []ColumnDefinition{},
		ForeignKeys: []ForeignKeyDefinition{},
		Indexes:     []IndexDefinition{},
	}

	// 1. Fetch Primary Key
	const pkSQLStatement = `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		LIMIT 1
	`
	_ = engine.db.QueryRow(ctx, pkSQLStatement, schema, table).Scan(&tableSummary.PrimaryKey)

	// 2. Fetch Columns
	const columnSQLStatement = `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position ASC
	`
	columnRows, queryColumnErr := engine.db.Query(ctx, columnSQLStatement, schema, table)
	if queryColumnErr == nil {
		defer columnRows.Close()
		for columnRows.Next() {
			var columnName, dataType, isNullable string
			var columnDefault *string
			if columnScanErr := columnRows.Scan(&columnName, &dataType, &isNullable, &columnDefault); columnScanErr == nil {
				tableSummary.Columns = append(tableSummary.Columns, ColumnDefinition{
					Name:         columnName,
					Type:         dataType,
					IsNullable:   isNullable == "YES",
					DefaultValue: columnDefault,
					IsPrimaryKey: columnName == tableSummary.PrimaryKey,
				})
			}
		}
	}

	// 3. Fetch Foreign Keys
	const fkSQLStatement = `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_schema AS foreign_schema,
			ccu.table_name AS foreign_table,
			ccu.column_name AS foreign_column,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc
		  ON rc.constraint_name = tc.constraint_name AND rc.constraint_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = $1 AND tc.table_name = $2
	`
	fkRows, fkErr := engine.db.Query(ctx, fkSQLStatement, schema, table)
	if fkErr == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var foreignKeyScanResult foreignKeyScanResult
			if fkScanErr := fkRows.Scan(&foreignKeyScanResult.ConstraintName, &foreignKeyScanResult.Column, &foreignKeyScanResult.ForeignSchema, &foreignKeyScanResult.ForeignTable, &foreignKeyScanResult.ForeignColumn, &foreignKeyScanResult.OnDelete, &foreignKeyScanResult.OnUpdate); fkScanErr == nil {
				tableSummary.ForeignKeys = append(tableSummary.ForeignKeys, ForeignKeyDefinition(foreignKeyScanResult))
			}
		}
	}

	// 4. Fetch Indexes
	indexes, _ := engine.ListIndexes(ctx, schema, table)
	tableSummary.Indexes = indexes

	// 5. Fetch RLS Status
	const rlsSQLStatement = `
		SELECT c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
	`
	_ = engine.db.QueryRow(ctx, rlsSQLStatement, schema, table).Scan(&tableSummary.RLSEnabled, &tableSummary.RLSForced)

	return tableSummary, nil
}

type foreignKeyScanResult ForeignKeyDefinition

// CreateTable generates and executes CREATE TABLE with standard UUIDv7 primary key default.
func (engine *DDLEngine) CreateTable(ctx context.Context, createTableRequest CreateTableRequest) error {
	schema := createTableRequest.Schema
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot create tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(createTableRequest.Name) {
		return fmt.Errorf("invalid table name '%s'", createTableRequest.Name)
	}

	primaryKeyName := createTableRequest.PrimaryKeyName
	if primaryKeyName == "" {
		primaryKeyName = "id"
	}
	if !common.IsValidIdentifier(primaryKeyName) {
		return fmt.Errorf("invalid primary key name '%s'", primaryKeyName)
	}

	var columnClauses []string

	hasPrimaryKey := false
	for _, columnDefinition := range createTableRequest.Columns {
		if !common.IsValidIdentifier(columnDefinition.Name) {
			return fmt.Errorf("invalid column name '%s'", columnDefinition.Name)
		}
		if columnDefinition.IsPrimaryKey || columnDefinition.Name == primaryKeyName {
			hasPrimaryKey = true
			columnClauses = append(columnClauses, fmt.Sprintf("%s UUID PRIMARY KEY DEFAULT uuidv7()", quoteIdent(columnDefinition.Name)))
			continue
		}

		clause := buildColumnClause(columnDefinition)
		columnClauses = append(columnClauses, clause)
	}

	if !hasPrimaryKey {
		columnClauses = append([]string{fmt.Sprintf("%s UUID PRIMARY KEY DEFAULT uuidv7()", quoteIdent(primaryKeyName))}, columnClauses...)
	}

	for _, foreignKey := range createTableRequest.ForeignKeys {
		if !common.IsValidIdentifier(foreignKey.Column) || !common.IsValidIdentifier(foreignKey.ForeignTable) || !common.IsValidIdentifier(foreignKey.ForeignColumn) {
			return fmt.Errorf("invalid foreign key identifiers")
		}
		foreignSchema := foreignKey.ForeignSchema
		if foreignSchema == "" {
			foreignSchema = schema
		}
		onDeleteAction := ""
		if foreignKey.OnDelete != "" {
			onDeleteAction = " ON DELETE " + sanitizeAction(foreignKey.OnDelete)
		}
		onUpdateAction := ""
		if foreignKey.OnUpdate != "" {
			onUpdateAction = " ON UPDATE " + sanitizeAction(foreignKey.OnUpdate)
		}
		columnClauses = append(columnClauses, fmt.Sprintf(
			"FOREIGN KEY (%s) REFERENCES %s.%s(%s)%s%s",
			quoteIdent(foreignKey.Column), quoteIdent(foreignSchema), quoteIdent(foreignKey.ForeignTable), quoteIdent(foreignKey.ForeignColumn), onDeleteAction, onUpdateAction,
		))
	}

	createSQLStatement := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (\n    %s\n);", quoteIdent(schema), quoteIdent(createTableRequest.Name), strings.Join(columnClauses, ",\n    "))
	if _, execErr := engine.db.Exec(ctx, createSQLStatement); execErr != nil {
		return fmt.Errorf("failed to create table %s.%s: %w", schema, createTableRequest.Name, execErr)
	}

	return nil
}

// DropTable drops a user table with optional CASCADE.
func (engine *DDLEngine) DropTable(ctx context.Context, schema, table string, cascade bool) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot drop tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid table name '%s'", table)
	}

	cascadeAction := ""
	if cascade {
		cascadeAction = " CASCADE"
	}

	dropSQLStatement := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s%s;", quoteIdent(schema), quoteIdent(table), cascadeAction)
	if _, execErr := engine.db.Exec(ctx, dropSQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop table %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// TruncateTable empties all records from a table.
func (engine *DDLEngine) TruncateTable(ctx context.Context, schema, table string, cascade bool) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot truncate tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid table name '%s'", table)
	}

	cascadeAction := ""
	if cascade {
		cascadeAction = " CASCADE"
	}

	truncateSQLStatement := fmt.Sprintf("TRUNCATE TABLE %s.%s%s;", quoteIdent(schema), quoteIdent(table), cascadeAction)
	if _, execErr := engine.db.Exec(ctx, truncateSQLStatement); execErr != nil {
		return fmt.Errorf("failed to truncate table %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// AddColumn adds a column to an existing table.
func (engine *DDLEngine) AddColumn(ctx context.Context, schema, table string, columnDefinition ColumnDefinition) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(columnDefinition.Name) {
		return fmt.Errorf("invalid table or column name")
	}

	clause := buildColumnClause(columnDefinition)
	alterSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s;", quoteIdent(schema), quoteIdent(table), clause)
	if _, execErr := engine.db.Exec(ctx, alterSQLStatement); execErr != nil {
		return fmt.Errorf("failed to add column to %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// AlterColumn alters column type, nullability, default expression, or name.
func (engine *DDLEngine) AlterColumn(ctx context.Context, schema, table, columnName string, alterColumnRequest AlterColumnRequest) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(columnName) {
		return fmt.Errorf("invalid table or column name")
	}

	var alterStatements []string

	// Rename column
	if alterColumnRequest.NewName != nil && *alterColumnRequest.NewName != "" && *alterColumnRequest.NewName != columnName {
		if !common.IsValidIdentifier(*alterColumnRequest.NewName) {
			return fmt.Errorf("invalid new column name '%s'", *alterColumnRequest.NewName)
		}
		alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s;",
			quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), quoteIdent(*alterColumnRequest.NewName)))
		columnName = *alterColumnRequest.NewName
	}

	// Change Type
	if alterColumnRequest.NewType != nil && *alterColumnRequest.NewType != "" {
		safeType := sanitizeType(*alterColumnRequest.NewType)
		alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s TYPE %s USING %s::%s;",
			quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), safeType, quoteIdent(columnName), safeType))
	}

	// Change Nullability
	if alterColumnRequest.IsNullable != nil {
		if *alterColumnRequest.IsNullable {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP NOT NULL;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		} else {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET NOT NULL;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		}
	}

	// Change Default
	if alterColumnRequest.DefaultValue != nil {
		if *alterColumnRequest.DefaultValue == "" {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP DEFAULT;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		} else {
			safeDef, defaultErr := sanitizeDefaultValue(*alterColumnRequest.DefaultValue)
			if defaultErr != nil {
				return defaultErr
			}
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET DEFAULT %s;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), safeDef))
		}
	}

	if len(alterStatements) == 0 {
		return errors.New("no modifications provided in alter column request")
	}

	tx, txErr := engine.db.Begin(ctx)
	if txErr != nil {
		return txErr
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, statement := range alterStatements {
		if _, execErr := tx.Exec(ctx, statement); execErr != nil {
			return fmt.Errorf("failed to alter column: %w", execErr)
		}
	}

	return tx.Commit(ctx)
}

// DropColumn drops a column from an existing table.
func (engine *DDLEngine) DropColumn(ctx context.Context, schema, table, columnName string, cascade bool) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(columnName) {
		return fmt.Errorf("invalid table or column name")
	}

	cascadeAction := ""
	if cascade {
		cascadeAction = " CASCADE"
	}

	dropSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s%s;", quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), cascadeAction)
	if _, execErr := engine.db.Exec(ctx, dropSQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop column %s from %s.%s: %w", columnName, schema, table, execErr)
	}
	return nil
}

// ListIndexes returns all indexes created on the specified table.
func (engine *DDLEngine) ListIndexes(ctx context.Context, schema, table string) ([]IndexDefinition, error) {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return nil, fmt.Errorf("schema '%s' is protected", schema)
	}
	if !common.IsValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name '%s'", table)
	}

	const indexesSQLStatement = `
		SELECT
			i.relname AS index_name,
			am.amname AS index_type,
			ix.indisunique AS is_unique,
			ARRAY_AGG(a.attname ORDER BY array_position(ix.indkey, a.attnum)) AS columns
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = i.relam
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = $1 AND t.relname = $2
		GROUP BY i.relname, am.amname, ix.indisunique
		ORDER BY i.relname ASC
	`
	rows, queryErr := engine.db.Query(ctx, indexesSQLStatement, schema, table)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", queryErr)
	}
	defer rows.Close()

	var indexes []IndexDefinition
	for rows.Next() {
		var indexDefinition IndexDefinition
		if scanErr := rows.Scan(&indexDefinition.IndexName, &indexDefinition.Type, &indexDefinition.IsUnique, &indexDefinition.Columns); scanErr == nil {
			indexes = append(indexes, indexDefinition)
		}
	}
	return indexes, nil
}

// CreateIndex creates an index on the table.
func (engine *DDLEngine) CreateIndex(ctx context.Context, schema, table string, createIndexRequest CreateIndexRequest) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot create index in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || len(createIndexRequest.Columns) == 0 {
		return fmt.Errorf("table and at least one column required")
	}

	indexName := createIndexRequest.IndexName
	if indexName == "" {
		indexName = fmt.Sprintf("idx_%s_%s_%s", schema, table, strings.Join(createIndexRequest.Columns, "_"))
	}
	if !common.IsValidIdentifier(indexName) {
		return fmt.Errorf("invalid index name '%s'", indexName)
	}

	var quotedColumns []string
	for _, columnName := range createIndexRequest.Columns {
		if !common.IsValidIdentifier(columnName) {
			return fmt.Errorf("invalid index column '%s'", columnName)
		}
		quotedColumns = append(quotedColumns, quoteIdent(columnName))
	}

	indexType := strings.ToLower(createIndexRequest.Type)
	if indexType == "" {
		indexType = "btree"
	}
	switch indexType {
	case "btree", "gin", "gist", "brin", "hash":
	default:
		return fmt.Errorf("unsupported index type '%s'", indexType)
	}

	uniqueClause := ""
	if createIndexRequest.IsUnique {
		uniqueClause = "UNIQUE "
	}

	createIndexSQLStatement := fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s.%s USING %s (%s);",
		uniqueClause, quoteIdent(indexName), quoteIdent(schema), quoteIdent(table), indexType, strings.Join(quotedColumns, ", "))
	if _, execErr := engine.db.Exec(ctx, createIndexSQLStatement); execErr != nil {
		return fmt.Errorf("failed to create index %s: %w", indexName, execErr)
	}
	return nil
}

// DropIndex drops an index.
func (engine *DDLEngine) DropIndex(ctx context.Context, schema, indexName string) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot drop index in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(indexName) {
		return fmt.Errorf("invalid index name '%s'", indexName)
	}

	dropIndexSQLStatement := fmt.Sprintf("DROP INDEX IF EXISTS %s.%s;", quoteIdent(schema), quoteIdent(indexName))
	if _, execErr := engine.db.Exec(ctx, dropIndexSQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop index %s: %w", indexName, execErr)
	}
	return nil
}

// ExecuteSQL executes arbitrary SQL query or DDL script for Layr Console SQL Scratchpad.
func (engine *DDLEngine) ExecuteSQL(ctx context.Context, sqlQuery string) (*ExecuteSQLResponse, error) {
	trimmed := strings.TrimSpace(sqlQuery)
	if trimmed == "" {
		return nil, errors.New("empty SQL query")
	}

	start := time.Now()
	rows, queryErr := engine.db.Query(ctx, trimmed)
	if queryErr != nil {
		return nil, fmt.Errorf("SQL execution error: %w", queryErr)
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	var columnNames []string
	for _, fieldDescription := range fieldDescriptions {
		columnNames = append(columnNames, fieldDescription.Name)
	}

	var rowData []map[string]any
	for rows.Next() {
		values, valueErr := rows.Values()
		if valueErr == nil {
			rowMap := make(map[string]any)
			for i, columnName := range columnNames {
				rowMap[columnName] = values[i]
			}
			rowData = append(rowData, rowMap)
		}
	}

	rowsAffected := int64(len(rowData))
	if commandResult := rows.CommandTag(); commandResult.RowsAffected() > 0 {
		rowsAffected = commandResult.RowsAffected()
	}

	const microsecondsPerMillisecond = 1000.0
	return &ExecuteSQLResponse{
		Columns:         columnNames,
		Rows:            rowData,
		RowsAffected:    rowsAffected,
		ExecutionTimeMs: float64(time.Since(start).Microseconds()) / microsecondsPerMillisecond,
	}, nil
}

// EnableRLS enables Row-Level Security on the specified table.
func (engine *DDLEngine) EnableRLS(ctx context.Context, schema, table string) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter table in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid table name '%s'", table)
	}

	enableRLSSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s ENABLE ROW LEVEL SECURITY;", quoteIdent(schema), quoteIdent(table))
	if _, execErr := engine.db.Exec(ctx, enableRLSSQLStatement); execErr != nil {
		return fmt.Errorf("failed to enable RLS on %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// DisableRLS disables Row-Level Security on the specified table.
func (engine *DDLEngine) DisableRLS(ctx context.Context, schema, table string) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter table in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid table name '%s'", table)
	}

	disableRLSSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s DISABLE ROW LEVEL SECURITY;", quoteIdent(schema), quoteIdent(table))
	if _, execErr := engine.db.Exec(ctx, disableRLSSQLStatement); execErr != nil {
		return fmt.Errorf("failed to disable RLS on %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// ForceRLS forces Row-Level Security even for table owners.
func (engine *DDLEngine) ForceRLS(ctx context.Context, schema, table string, force bool) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter table in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid table name '%s'", table)
	}

	action := "FORCE"
	if !force {
		action = "NO FORCE"
	}
	forceRLSSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s %s ROW LEVEL SECURITY;", quoteIdent(schema), quoteIdent(table), action)
	if _, execErr := engine.db.Exec(ctx, forceRLSSQLStatement); execErr != nil {
		return fmt.Errorf("failed to set %s ROW LEVEL SECURITY on %s.%s: %w", action, schema, table, execErr)
	}
	return nil
}

// ListPolicies returns all RLS policies for the specified table.
func (engine *DDLEngine) ListPolicies(ctx context.Context, schema, table string) ([]PolicyDefinition, error) {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return nil, fmt.Errorf("cannot view policies in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name '%s'", table)
	}

	const policiesSQLStatement = `
		SELECT 
			policyname,
			permissive,
			cmd,
			roles::text[],
			COALESCE(qual, ''),
			COALESCE(with_check, '')
		FROM pg_policies
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY policyname ASC
	`
	rows, queryErr := engine.db.Query(ctx, policiesSQLStatement, schema, table)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to list policies: %w", queryErr)
	}
	defer rows.Close()

	var policies []PolicyDefinition
	for rows.Next() {
		policyDefinition := PolicyDefinition{Schema: schema, Table: table}
		if scanErr := rows.Scan(&policyDefinition.Name, &policyDefinition.Permissive, &policyDefinition.Command, &policyDefinition.Roles, &policyDefinition.UsingExpression, &policyDefinition.CheckExpression); scanErr == nil {
			policies = append(policies, policyDefinition)
		}
	}
	return policies, nil
}

// CreatePolicy creates a Row-Level Security policy on a table.
func (engine *DDLEngine) CreatePolicy(ctx context.Context, schema, table string, createPolicyRequest CreatePolicyRequest) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot manage policies in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(createPolicyRequest.Name) {
		return fmt.Errorf("invalid table or policy name")
	}

	policyCommand := strings.ToUpper(strings.TrimSpace(createPolicyRequest.Command))
	if policyCommand == "" {
		policyCommand = "ALL"
	}
	switch policyCommand {
	case "ALL", "SELECT", "INSERT", "UPDATE", "DELETE":
	default:
		return fmt.Errorf("invalid policy command '%s', must be ALL, SELECT, INSERT, UPDATE, or DELETE", policyCommand)
	}

	permissive := strings.ToUpper(strings.TrimSpace(createPolicyRequest.Permissive))
	if permissive == "" {
		permissive = "PERMISSIVE"
	}
	switch permissive {
	case "PERMISSIVE", "RESTRICTIVE":
	default:
		return fmt.Errorf("invalid policy type '%s', must be PERMISSIVE or RESTRICTIVE", permissive)
	}

	var roles []string
	if len(createPolicyRequest.Roles) == 0 {
		roles = []string{"PUBLIC"}
	} else {
		for _, roleName := range createPolicyRequest.Roles {
			trimmed := strings.TrimSpace(roleName)
			if strings.EqualFold(trimmed, "public") {
				roles = append(roles, "PUBLIC")
			} else if common.IsValidIdentifier(trimmed) {
				roles = append(roles, quoteIdent(trimmed))
			} else {
				return fmt.Errorf("invalid role name '%s'", roleName)
			}
		}
	}

	var clauses []string
	clauses = append(clauses, fmt.Sprintf("CREATE POLICY %s ON %s.%s AS %s FOR %s TO %s",
		quoteIdent(createPolicyRequest.Name), quoteIdent(schema), quoteIdent(table), permissive, policyCommand, strings.Join(roles, ", ")))

	if createPolicyRequest.UsingExpression != "" {
		if strings.Contains(createPolicyRequest.UsingExpression, ";") || strings.Contains(createPolicyRequest.UsingExpression, "--") || strings.Contains(createPolicyRequest.UsingExpression, "/*") {
			return fmt.Errorf("using expression contains forbidden characters")
		}
		clauses = append(clauses, fmt.Sprintf("USING (%s)", createPolicyRequest.UsingExpression))
	}
	if createPolicyRequest.CheckExpression != "" {
		if strings.Contains(createPolicyRequest.CheckExpression, ";") || strings.Contains(createPolicyRequest.CheckExpression, "--") || strings.Contains(createPolicyRequest.CheckExpression, "/*") {
			return fmt.Errorf("check expression contains forbidden characters")
		}
		clauses = append(clauses, fmt.Sprintf("WITH CHECK (%s)", createPolicyRequest.CheckExpression))
	}

	policySQLStatement := strings.Join(clauses, " ") + ";"
	if _, execErr := engine.db.Exec(ctx, policySQLStatement); execErr != nil {
		return fmt.Errorf("failed to create policy '%s' on %s.%s: %w", createPolicyRequest.Name, schema, table, execErr)
	}
	return nil
}

// DropPolicy drops a Row-Level Security policy from a table.
func (engine *DDLEngine) DropPolicy(ctx context.Context, schema, table, policyName string) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot manage policies in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(policyName) {
		return fmt.Errorf("invalid table or policy name")
	}

	dropPolicySQLStatement := fmt.Sprintf("DROP POLICY IF EXISTS %s ON %s.%s;", quoteIdent(policyName), quoteIdent(schema), quoteIdent(table))
	if _, execErr := engine.db.Exec(ctx, dropPolicySQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop policy '%s' on %s.%s: %w", policyName, schema, table, execErr)
	}
	return nil
}

func buildColumnClause(columnDefinition ColumnDefinition) string {
	safeType := sanitizeType(columnDefinition.Type)
	var parts []string
	parts = append(parts, quoteIdent(columnDefinition.Name), safeType)

	if !columnDefinition.IsNullable {
		parts = append(parts, "NOT NULL")
	}
	if columnDefinition.DefaultValue != nil && *columnDefinition.DefaultValue != "" {
		if safeDef, err := sanitizeDefaultValue(*columnDefinition.DefaultValue); err == nil {
			parts = append(parts, "DEFAULT "+safeDef)
		}
	}
	if columnDefinition.IsUnique {
		parts = append(parts, "UNIQUE")
	}

	return strings.Join(parts, " ")
}

func sanitizeDefaultValue(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("empty default value")
	}
	if strings.Contains(trimmed, ";") || strings.Contains(trimmed, "--") || strings.Contains(trimmed, "/*") {
		return "", fmt.Errorf("default value expression contains forbidden characters: %q", raw)
	}
	return trimmed, nil
}

func sanitizeType(raw string) string {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	switch lower {
	case "uuid", "text", "varchar", "integer", "int", "bigint", "smallint", "boolean", "bool",
		"timestamptz", "timestamp", "timestamp with time zone", "timestamp without time zone",
		"date", "time", "jsonb", "json", "numeric", "float8", "real", "bytea":
		return lower
	default:
		if strings.HasPrefix(lower, "varchar(") && strings.HasSuffix(lower, ")") {
			return lower
		}
		if strings.HasPrefix(lower, "numeric(") && strings.HasSuffix(lower, ")") {
			return lower
		}
		return "text"
	}
}

func sanitizeAction(action string) string {
	upper := strings.ToUpper(strings.TrimSpace(action))
	switch upper {
	case "CASCADE", "SET NULL", "RESTRICT", "NO ACTION", "SET DEFAULT":
		return upper
	default:
		return "NO ACTION"
	}
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
