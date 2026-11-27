package data

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
	"layr.sh/data/common"
)

// IsProtectedSchema returns true if the schema is reserved for platform internals or PostgreSQL catalog.
func IsProtectedSchema(schema string) bool {
	return common.IsProtectedSchema(schema)
}

// DDLEngine executes schema introspection and DDL commands.
type DDLEngine struct {
	kernel *core.Kernel
}

// NewDDLEngine initializes the DDL schema engine.
func NewDDLEngine(kernel *core.Kernel) *DDLEngine {
	return &DDLEngine{kernel: kernel}
}

// ListTables retrieves metadata for all user tables across the specified schemas.
func (engine *DDLEngine) ListTables(ctx context.Context, schemas []string) ([]Table, error) {
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	var results []Table
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
		rows, queryErr := engine.kernel.DB().Query(ctx, listSQLStatement, schema)
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
			if scanErr := rows.Scan(&statsRecord.name, &statsRecord.estRows, &statsRecord.sizeBytes); scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			tableList = append(tableList, statsRecord)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		for _, statsRecord := range tableList {
			table, err := engine.GetTable(ctx, schema, statsRecord.name)
			if err != nil {
				return nil, err
			}
			if table != nil {
				table.EstimatedRowCount = statsRecord.estRows
				table.SizeBytes = statsRecord.sizeBytes
				results = append(results, *table)
			}
		}
	}

	return results, nil
}

// GetTable retrieves detailed column, foreign key, and index metadata for a table.
func (engine *DDLEngine) GetTable(ctx context.Context, schema, tableName string) (*Table, error) {
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
	scanErr := engine.kernel.DB().QueryRow(ctx, existsSQLStatement, schema, tableName).Scan(&exists)
	if scanErr != nil {
		return nil, scanErr
	}
	if !exists {
		return nil, &pgconn.PgError{Code: "42P01", Message: fmt.Sprintf("table %s.%s not found", schema, tableName)}
	}

	table := &Table{
		Schema:      schema,
		Name:        tableName,
		Columns:     []Column{},
		ForeignKeys: []ForeignKey{},
		Indexes:     []Index{},
	}

	// 1. Fetch Primary Key
	const pkSQLStatement = `
		SELECT min(kcu.column_name::text)
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema AND tc.table_name = kcu.table_name
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		HAVING count(*) = 1
	`
	if err := engine.kernel.DB().QueryRow(ctx, pkSQLStatement, schema, tableName).Scan(&table.PrimaryKey); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 2. Fetch Columns
	const columnSQLStatement = `
		SELECT a.attname, format_type(a.atttypid, a.atttypmod), CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END, pg_get_expr(d.adbin, d.adrelid),
 EXISTS(SELECT 1 FROM pg_index i WHERE i.indrelid=c.oid AND i.indisprimary AND a.attnum=ANY(i.indkey)),
 EXISTS(SELECT 1 FROM pg_index i WHERE i.indrelid=c.oid AND i.indisunique AND i.indnkeyatts=1 AND i.indkey[0]=a.attnum AND i.indpred IS NULL)
 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
 WHERE n.nspname=$1 AND c.relname=$2 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum
	`
	columnRows, queryColumnErr := engine.kernel.DB().Query(ctx, columnSQLStatement, schema, tableName)
	if queryColumnErr != nil {
		return nil, queryColumnErr
	}
	{
		defer columnRows.Close()
		for columnRows.Next() {
			var columnName, dataType, isNullable string
			var columnDefault *string
			var primary, unique bool
			if columnScanErr := columnRows.Scan(&columnName, &dataType, &isNullable, &columnDefault, &primary, &unique); columnScanErr != nil {
				return nil, columnScanErr
			}
			{
				table.Columns = append(table.Columns, Column{
					Name:         columnName,
					Type:         dataType,
					IsNullable:   isNullable == "YES",
					DefaultValue: columnDefault,
					IsPrimaryKey: primary,
					IsUnique:     unique,
				})
			}
		}
	}

	columnRows.Close()
	if err := columnRows.Err(); err != nil {
		return nil, err
	}

	// 3. Fetch Foreign Keys
	const fkSQLStatement = `SELECT con.conname, source.attname, target_schema.nspname, target_table.relname, target.attname,
 CASE con.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL' WHEN 'd' THEN 'SET DEFAULT' END,
 CASE con.confupdtype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL' WHEN 'd' THEN 'SET DEFAULT' END
 FROM pg_constraint con
 JOIN pg_class source_table ON source_table.oid=con.conrelid JOIN pg_namespace source_schema ON source_schema.oid=source_table.relnamespace
 JOIN pg_class target_table ON target_table.oid=con.confrelid JOIN pg_namespace target_schema ON target_schema.oid=target_table.relnamespace
 CROSS JOIN LATERAL unnest(con.conkey, con.confkey) AS keys(source_num, target_num)
 JOIN pg_attribute source ON source.attrelid=source_table.oid AND source.attnum=keys.source_num
 JOIN pg_attribute target ON target.attrelid=target_table.oid AND target.attnum=keys.target_num
 WHERE con.contype='f' AND source_schema.nspname=$1 AND source_table.relname=$2 ORDER BY con.conname, source.attnum`
	fkRows, fkErr := engine.kernel.DB().Query(ctx, fkSQLStatement, schema, tableName)
	if fkErr != nil {
		return nil, fkErr
	}
	{
		defer fkRows.Close()
		for fkRows.Next() {
			var foreignKeyScanResult foreignKeyScanResult
			if fkScanErr := fkRows.Scan(&foreignKeyScanResult.ConstraintName, &foreignKeyScanResult.Column, &foreignKeyScanResult.ForeignSchema, &foreignKeyScanResult.ForeignTable, &foreignKeyScanResult.ForeignColumn, &foreignKeyScanResult.OnDelete, &foreignKeyScanResult.OnUpdate); fkScanErr != nil {
				return nil, fkScanErr
			}
			{
				table.ForeignKeys = append(table.ForeignKeys, ForeignKey(foreignKeyScanResult))
			}
		}
	}

	// 4. Fetch Indexes
	fkRows.Close()
	if err := fkRows.Err(); err != nil {
		return nil, err
	}
	indexes, err := engine.ListIndexes(ctx, schema, tableName)
	if err != nil {
		return nil, err
	}
	table.Indexes = indexes

	// 5. Fetch RLS Status
	const rlsSQLStatement = `
		SELECT c.relrowsecurity, c.relforcerowsecurity, c.reltuples::bigint, pg_total_relation_size(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
	`
	if err := engine.kernel.DB().QueryRow(ctx, rlsSQLStatement, schema, tableName).Scan(&table.RLSEnabled, &table.RLSForced, &table.EstimatedRowCount, &table.SizeBytes); err != nil {
		return nil, err
	}

	return table, nil
}

type foreignKeyScanResult ForeignKey

// CreateTable generates and executes CREATE TABLE with standard UUIDv7 primary key default.
func (engine *DDLEngine) CreateTable(ctx context.Context, createTableInput CreateTableInput) error {
	schema := createTableInput.Schema
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot create tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(createTableInput.Name) {
		return fmt.Errorf("invalid table name '%s'", createTableInput.Name)
	}

	primaryKeyName := createTableInput.PrimaryKeyName
	if primaryKeyName == "" {
		primaryKeyName = "id"
	}
	if !common.IsValidIdentifier(primaryKeyName) {
		return fmt.Errorf("invalid primary key name '%s'", primaryKeyName)
	}

	var columnClauses []string

	hasPrimaryKey := false
	for _, column := range createTableInput.Columns {
		if !common.IsValidIdentifier(column.Name) {
			return fmt.Errorf("invalid column name '%s'", column.Name)
		}
		if column.IsPrimaryKey || column.Name == primaryKeyName {
			hasPrimaryKey = true
			columnClauses = append(columnClauses, fmt.Sprintf("%s UUID PRIMARY KEY DEFAULT uuidv7()", quoteIdent(column.Name)))
			continue
		}

		if err := validateColumnDefinition(column); err != nil {
			return err
		}
		clause := buildColumnClause(column)
		columnClauses = append(columnClauses, clause)
	}

	if !hasPrimaryKey {
		columnClauses = append([]string{fmt.Sprintf("%s UUID PRIMARY KEY DEFAULT uuidv7()", quoteIdent(primaryKeyName))}, columnClauses...)
	}

	for _, foreignKey := range createTableInput.ForeignKeys {
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

	createSQLStatement := fmt.Sprintf("CREATE TABLE %s.%s (\n    %s\n);", quoteIdent(schema), quoteIdent(createTableInput.Name), strings.Join(columnClauses, ",\n    "))
	if _, execErr := engine.kernel.DB().Exec(ctx, createSQLStatement); execErr != nil {
		return fmt.Errorf("failed to create table %s.%s: %w", schema, createTableInput.Name, execErr)
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
	if _, execErr := engine.kernel.DB().Exec(ctx, dropSQLStatement); execErr != nil {
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
	if _, execErr := engine.kernel.DB().Exec(ctx, truncateSQLStatement); execErr != nil {
		return fmt.Errorf("failed to truncate table %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// AddColumn adds a column to an existing table.
func (engine *DDLEngine) AddColumn(ctx context.Context, schema, table string, column Column) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot alter tables in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(column.Name) {
		return fmt.Errorf("invalid table or column name")
	}

	if err := validateColumnDefinition(column); err != nil {
		return err
	}
	clause := buildColumnClause(column)
	addColumnSQLStatement := fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s;", quoteIdent(schema), quoteIdent(table), clause)
	if _, execErr := engine.kernel.DB().Exec(ctx, addColumnSQLStatement); execErr != nil {
		return fmt.Errorf("failed to add column to %s.%s: %w", schema, table, execErr)
	}
	return nil
}

// AlterColumn alters column type, nullability, default expression, or name.
func (engine *DDLEngine) AlterColumn(ctx context.Context, schema, table, columnName string, updateColumnInput UpdateColumnInput) error {
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
	if updateColumnInput.NewName != nil && *updateColumnInput.NewName != "" && *updateColumnInput.NewName != columnName {
		if !common.IsValidIdentifier(*updateColumnInput.NewName) {
			return fmt.Errorf("invalid new column name '%s'", *updateColumnInput.NewName)
		}
		alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s;",
			quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), quoteIdent(*updateColumnInput.NewName)))
		columnName = *updateColumnInput.NewName
	}

	// Change Type
	if updateColumnInput.NewType != nil && *updateColumnInput.NewType != "" {
		safeType := sanitizeType(*updateColumnInput.NewType)
		if safeType == "" {
			return fmt.Errorf("unsupported column type %q", *updateColumnInput.NewType)
		}
		alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s TYPE %s USING %s::%s;",
			quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), safeType, quoteIdent(columnName), safeType))
	}

	// Change Nullability
	if updateColumnInput.IsNullable != nil {
		if *updateColumnInput.IsNullable {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP NOT NULL;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		} else {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET NOT NULL;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		}
	}

	// Change Default
	if updateColumnInput.DefaultValue != nil {
		if *updateColumnInput.DefaultValue == "" {
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP DEFAULT;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName)))
		} else {
			safeDef, err := sanitizeDefaultValue(*updateColumnInput.DefaultValue)
			if err != nil {
				return err
			}
			alterStatements = append(alterStatements, fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET DEFAULT %s;",
				quoteIdent(schema), quoteIdent(table), quoteIdent(columnName), safeDef))
		}
	}

	if len(alterStatements) == 0 {
		return errors.New("no modifications provided in alter column request")
	}

	tx, txErr := engine.kernel.DB().Begin(ctx)
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
	if _, execErr := engine.kernel.DB().Exec(ctx, dropSQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop column %s from %s.%s: %w", columnName, schema, table, execErr)
	}
	return nil
}

// ListIndexes returns all indexes created on the specified table.
func (engine *DDLEngine) ListIndexes(ctx context.Context, schema, table string) ([]Index, error) {
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
			ARRAY(SELECT pg_get_indexdef(ix.indexrelid, position, true)
							FROM generate_series(1, ix.indnatts) AS position ORDER BY position) AS columns
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = i.relam

		WHERE n.nspname = $1 AND t.relname = $2

		ORDER BY i.relname ASC
	`
	rows, queryErr := engine.kernel.DB().Query(ctx, indexesSQLStatement, schema, table)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", queryErr)
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var index Index
		if scanErr := rows.Scan(&index.IndexName, &index.Type, &index.IsUnique, &index.Columns); scanErr != nil {
			return nil, scanErr
		}
		indexes = append(indexes, index)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return indexes, nil
}

// CreateIndex creates an index on the table.
func (engine *DDLEngine) CreateIndex(ctx context.Context, schema, table string, createIndexInput CreateIndexInput) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot create index in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || len(createIndexInput.Columns) == 0 {
		return fmt.Errorf("table and at least one column required")
	}

	indexName := createIndexInput.IndexName
	if indexName == "" {
		indexName = fmt.Sprintf("idx_%s_%s_%s", schema, table, strings.Join(createIndexInput.Columns, "_"))
	}
	if !common.IsValidIdentifier(indexName) {
		return fmt.Errorf("invalid index name '%s'", indexName)
	}

	var quotedColumns []string
	for _, columnName := range createIndexInput.Columns {
		if !common.IsValidIdentifier(columnName) {
			return fmt.Errorf("invalid index column '%s'", columnName)
		}
		quotedColumns = append(quotedColumns, quoteIdent(columnName))
	}

	indexType := strings.ToLower(createIndexInput.Type)
	if indexType == "" {
		indexType = "btree"
	}
	switch indexType {
	case "btree", "gin", "gist", "brin", "hash":
	default:
		return fmt.Errorf("unsupported index type '%s'", indexType)
	}

	uniqueClause := ""
	if createIndexInput.IsUnique {
		uniqueClause = "UNIQUE "
	}

	createIndexSQLStatement := fmt.Sprintf("CREATE %sINDEX %s ON %s.%s USING %s (%s);",
		uniqueClause, quoteIdent(indexName), quoteIdent(schema), quoteIdent(table), indexType, strings.Join(quotedColumns, ", "))
	if _, execErr := engine.kernel.DB().Exec(ctx, createIndexSQLStatement); execErr != nil {
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
	if _, execErr := engine.kernel.DB().Exec(ctx, dropIndexSQLStatement); execErr != nil {
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

	if err := validateSQLTransaction(trimmed); err != nil {
		return nil, err
	}
	start := time.Now()
	connection, acquireErr := engine.kernel.DB().Acquire(ctx)
	if acquireErr != nil {
		return nil, fmt.Errorf("SQL execution error: %w", acquireErr)
	}
	// Scratchpad statements may change session settings, temporary objects, or
	// advisory locks. Never return that session to the application query pool.
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = connection.Conn().Close(closeCtx)
		connection.Release()
	}()
	tx, beginErr := connection.Begin(ctx)
	if beginErr != nil {
		return nil, fmt.Errorf("SQL execution error: %w", beginErr)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL standard_conforming_strings = on"); err != nil {
		return nil, err
	}
	rows, queryErr := tx.Query(ctx, trimmed, pgx.QueryExecModeSimpleProtocol)
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
		if valueErr != nil {
			return nil, fmt.Errorf("SQL result error: %w", valueErr)
		}
		rowMap := make(map[string]any)
		rawValues := rows.RawValues()
		for index, columnName := range columnNames {
			rowMap[columnName] = queryJSONColumn(values[index], fieldDescriptions[index], rawValues[index])
		}
		rowData = append(rowData, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SQL execution error: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("SQL commit error: %w", err)
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
	if _, execErr := engine.kernel.DB().Exec(ctx, enableRLSSQLStatement); execErr != nil {
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
	if _, execErr := engine.kernel.DB().Exec(ctx, disableRLSSQLStatement); execErr != nil {
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
	if _, execErr := engine.kernel.DB().Exec(ctx, forceRLSSQLStatement); execErr != nil {
		return fmt.Errorf("failed to set %s ROW LEVEL SECURITY on %s.%s: %w", action, schema, table, execErr)
	}
	return nil
}

// ListPolicies returns all RLS policies for the specified table.
func (engine *DDLEngine) ListPolicies(ctx context.Context, schema, table string) ([]Policy, error) {
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
	rows, queryErr := engine.kernel.DB().Query(ctx, policiesSQLStatement, schema, table)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to list policies: %w", queryErr)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		policy := Policy{Schema: schema, Table: table}
		if scanErr := rows.Scan(&policy.Name, &policy.Permissive, &policy.Command, &policy.Roles, &policy.UsingExpression, &policy.CheckExpression); scanErr == nil {
			policies = append(policies, policy)
		}
	}
	return policies, nil
}

// CreatePolicy creates a Row-Level Security policy on a table.
func (engine *DDLEngine) CreatePolicy(ctx context.Context, schema, table string, createPolicyInput CreatePolicyInput) error {
	if schema == "" {
		schema = "public"
	}
	if IsProtectedSchema(schema) {
		return fmt.Errorf("cannot manage policies in protected schema '%s'", schema)
	}
	if !common.IsValidIdentifier(table) || !common.IsValidIdentifier(createPolicyInput.Name) {
		return fmt.Errorf("invalid table or policy name")
	}

	policyCommand := strings.ToUpper(strings.TrimSpace(createPolicyInput.Command))
	if policyCommand == "" {
		policyCommand = "ALL"
	}
	switch policyCommand {
	case "ALL", "SELECT", "INSERT", "UPDATE", "DELETE":
	default:
		return fmt.Errorf("invalid policy command '%s', must be ALL, SELECT, INSERT, UPDATE, or DELETE", policyCommand)
	}

	permissive := strings.ToUpper(strings.TrimSpace(createPolicyInput.Permissive))
	if permissive == "" {
		permissive = "PERMISSIVE"
	}
	switch permissive {
	case "PERMISSIVE", "RESTRICTIVE":
	default:
		return fmt.Errorf("invalid policy type '%s', must be PERMISSIVE or RESTRICTIVE", permissive)
	}

	var roles []string
	if len(createPolicyInput.Roles) == 0 {
		roles = []string{"PUBLIC"}
	} else {
		for _, roleName := range createPolicyInput.Roles {
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
		quoteIdent(createPolicyInput.Name), quoteIdent(schema), quoteIdent(table), permissive, policyCommand, strings.Join(roles, ", ")))

	if createPolicyInput.UsingExpression != "" {
		if strings.Contains(createPolicyInput.UsingExpression, ";") || strings.Contains(createPolicyInput.UsingExpression, "--") || strings.Contains(createPolicyInput.UsingExpression, "/*") {
			return fmt.Errorf("using expression contains forbidden characters")
		}
		clauses = append(clauses, fmt.Sprintf("USING (%s)", createPolicyInput.UsingExpression))
	}
	if createPolicyInput.CheckExpression != "" {
		if strings.Contains(createPolicyInput.CheckExpression, ";") || strings.Contains(createPolicyInput.CheckExpression, "--") || strings.Contains(createPolicyInput.CheckExpression, "/*") {
			return fmt.Errorf("check expression contains forbidden characters")
		}
		clauses = append(clauses, fmt.Sprintf("WITH CHECK (%s)", createPolicyInput.CheckExpression))
	}

	policySQLStatement := strings.Join(clauses, " ") + ";"
	if _, execErr := engine.kernel.DB().Exec(ctx, policySQLStatement); execErr != nil {
		return fmt.Errorf("failed to create policy '%s' on %s.%s: %w", createPolicyInput.Name, schema, table, execErr)
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
	if _, execErr := engine.kernel.DB().Exec(ctx, dropPolicySQLStatement); execErr != nil {
		return fmt.Errorf("failed to drop policy '%s' on %s.%s: %w", policyName, schema, table, execErr)
	}
	return nil
}

func validateColumnDefinition(column Column) error {
	if sanitizeType(column.Type) == "" {
		return fmt.Errorf("unsupported column type %q", column.Type)
	}
	if column.DefaultValue != nil && *column.DefaultValue != "" {
		if _, err := sanitizeDefaultValue(*column.DefaultValue); err != nil {
			return err
		}
	}
	return nil
}

func buildColumnClause(column Column) string {
	safeType := sanitizeType(column.Type)
	var parts []string
	parts = append(parts, quoteIdent(column.Name), safeType)

	if !column.IsNullable {
		parts = append(parts, "NOT NULL")
	}
	if column.DefaultValue != nil && *column.DefaultValue != "" {
		if safeDef, err := sanitizeDefaultValue(*column.DefaultValue); err == nil {
			parts = append(parts, "DEFAULT "+safeDef)
		}
	}
	if column.IsUnique {
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

var parameterizedTypePattern = regexp.MustCompile(`^(varchar|character varying)\([1-9][0-9]*\)$|^numeric\([1-9][0-9]*(,\s*-?[0-9]+)?\)$`)

func sanitizeType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "text"
	}
	suffix := ""
	for strings.HasSuffix(value, "[]") {
		value = strings.TrimSpace(strings.TrimSuffix(value, "[]"))
		suffix += "[]"
	}
	switch value {
	case "uuid", "text", "varchar", "character varying", "integer", "int", "bigint", "smallint", "boolean", "bool", "timestamptz", "timestamp", "timestamp with time zone", "timestamp without time zone", "date", "time", "jsonb", "json", "numeric", "float8", "double precision", "real", "bytea":
		return value + suffix
	}
	if parameterizedTypePattern.MatchString(value) {
		return value + suffix
	}
	return ""
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
