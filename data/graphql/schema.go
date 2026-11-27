// Package graphql provides a single-shot GraphQL engine with schema introspection and caching.
package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// ColumnInfo holds details about a database column.
type ColumnInfo struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
}

// TableInfo holds metadata about a table in an exposed schema.
type TableInfo struct {
	Schema      string
	Name        string
	PrimaryKey  string
	Columns     map[string]ColumnInfo
	ForeignKeys map[string]RelationInfo // relation_name -> RelationInfo
}

// RelationInfo describes a foreign key relation.
type RelationInfo struct {
	RelationName  string
	ForeignSchema string
	ForeignTable  string
	LocalColumn   string
	TargetColumn  string
	IsArray       bool
}

// SchemaIntrospector caches schema information from PostgreSQL.
type SchemaIntrospector struct {
	kernel     *core.Kernel
	tables     map[string]*TableInfo // key: "schema.table"
	catalogTTL time.Duration
	rwMutex    sync.RWMutex
}

// NewSchemaIntrospector creates an introspector instance.
func NewSchemaIntrospector(kernel *core.Kernel) *SchemaIntrospector {
	return &SchemaIntrospector{
		kernel:     kernel,
		tables:     make(map[string]*TableInfo),
		catalogTTL: 1 * time.Hour,
	}
}

// SetCatalogTTL configures the TTL duration for cached schema metadata.
func (schemaInspector *SchemaIntrospector) SetCatalogTTL(ttl time.Duration) {
	schemaInspector.rwMutex.Lock()
	defer schemaInspector.rwMutex.Unlock()
	if ttl > 0 {
		schemaInspector.catalogTTL = ttl
	}
}

// SetTable sets metadata for a table (useful for testing and programmatic registration).
func (schemaInspector *SchemaIntrospector) SetTable(tableInfo *TableInfo) {
	if tableInfo == nil {
		return
	}
	schemaInspector.rwMutex.Lock()
	defer schemaInspector.rwMutex.Unlock()
	schema := tableInfo.Schema
	if schema == "" {
		schema = "public"
	}
	schemaInspector.tables[fmt.Sprintf("%s.%s", schema, tableInfo.Name)] = tableInfo
}

// GetTable returns table metadata if present.
func (schemaInspector *SchemaIntrospector) GetTable(schema, table string) (*TableInfo, bool) {
	schemaInspector.rwMutex.RLock()
	defer schemaInspector.rwMutex.RUnlock()

	if schema == "" {
		schema = "public"
	}
	tableKey := fmt.Sprintf("%s.%s", schema, table)
	tableInfo, ok := schemaInspector.tables[tableKey]
	return tableInfo, ok
}

// Tables returns a snapshot copy of all introspected tables.
func (schemaInspector *SchemaIntrospector) Tables() map[string]*TableInfo {
	schemaInspector.rwMutex.RLock()
	defer schemaInspector.rwMutex.RUnlock()

	snapshot := make(map[string]*TableInfo, len(schemaInspector.tables))
	for k, v := range schemaInspector.tables {
		snapshot[k] = v
	}
	return snapshot
}

// Introspect reads tables, columns, and foreign keys from PostgreSQL information_schema.
func (schemaInspector *SchemaIntrospector) Introspect(ctx context.Context, schemas []string) error {
	schemaInspector.rwMutex.RLock()
	catalogTTL := schemaInspector.catalogTTL
	schemaInspector.rwMutex.RUnlock()
	if cached, err := schemaInspector.kernel.KVStore().Get(ctx, "cache:schema:catalog"); err == nil && cached != "" {
		var cachedTables map[string]*TableInfo
		if err := json.Unmarshal([]byte(cached), &cachedTables); err == nil && len(cachedTables) > 0 {
			schemaInspector.rwMutex.Lock()
			schemaInspector.tables = cachedTables
			schemaInspector.rwMutex.Unlock()
			return nil
		}
	}

	newTables := make(map[string]*TableInfo)
	ambiguous := make(map[*TableInfo]map[string]bool)
	addRelation := func(tableInfo *TableInfo, alias string, relationInfo RelationInfo) {
		if ambiguous[tableInfo] == nil {
			ambiguous[tableInfo] = make(map[string]bool)
		}
		if ambiguous[tableInfo][alias] {
			return
		}
		if previousRelationInfo, exists := tableInfo.ForeignKeys[alias]; exists && previousRelationInfo != relationInfo {
			delete(tableInfo.ForeignKeys, alias)
			ambiguous[tableInfo][alias] = true
			return
		}
		tableInfo.ForeignKeys[alias] = relationInfo
	}

	for _, schema := range schemas {
		// Fetch tables
		rows, err := schemaInspector.kernel.DB().Query(ctx, `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		`, schema)
		if err != nil {
			return fmt.Errorf("failed to introspect tables in schema %s: %w", schema, err)
		}

		var tableNames []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			tableNames = append(tableNames, name)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, name := range tableNames {
			tableInfo := &TableInfo{
				Schema:      schema,
				Name:        name,
				PrimaryKey:  "",
				Columns:     make(map[string]ColumnInfo),
				ForeignKeys: make(map[string]RelationInfo),
			}

			// Fetch columns
			columnRows, queryColumnsErr := schemaInspector.kernel.DB().Query(ctx, `
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = $1 AND table_name = $2
			`, schema, name)
			if queryColumnsErr != nil {
				return queryColumnsErr
			}
			{
				for columnRows.Next() {
					var columnName, dataType, isNullable string
					if err := columnRows.Scan(&columnName, &dataType, &isNullable); err != nil {
						columnRows.Close()
						return err
					}
					{
						tableInfo.Columns[columnName] = ColumnInfo{
							Name:       columnName,
							DataType:   dataType,
							IsNullable: isNullable == "YES",
						}
					}
				}
				columnRows.Close()
				if err := columnRows.Err(); err != nil {
					return err
				}
			}

			// Fetch primary key
			var pkName string
			pkErr := schemaInspector.kernel.DB().QueryRow(ctx, `
				SELECT min(kcu.column_name::text)
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				  AND tc.table_schema = kcu.table_schema AND tc.table_name = kcu.table_name
				WHERE tc.constraint_type = 'PRIMARY KEY'
				  AND tc.table_schema = $1
				  AND tc.table_name = $2
				HAVING count(*) = 1
			`, schema, name).Scan(&pkName)
			if pkErr != nil && !errors.Is(pkErr, pgx.ErrNoRows) {
				return pkErr
			}
			if pkName != "" {
				tableInfo.PrimaryKey = pkName
				if columnInfo, ok := tableInfo.Columns[pkName]; ok {
					columnInfo.IsPrimaryKey = true
					tableInfo.Columns[pkName] = columnInfo
				}
			}

			// Fetch foreign keys (including cross-schema foreign keys)
			fkRows, queryFKErr := schemaInspector.kernel.DB().Query(ctx, `
SELECT source.attname, target_schema.nspname, target_table.relname, target.attname
 FROM pg_constraint con
 JOIN pg_class source_table ON source_table.oid=con.conrelid JOIN pg_namespace source_schema ON source_schema.oid=source_table.relnamespace
 JOIN pg_class target_table ON target_table.oid=con.confrelid JOIN pg_namespace target_schema ON target_schema.oid=target_table.relnamespace
 CROSS JOIN LATERAL unnest(con.conkey, con.confkey) AS keys(source_num, target_num)
 JOIN pg_attribute source ON source.attrelid=source_table.oid AND source.attnum=keys.source_num
 JOIN pg_attribute target ON target.attrelid=target_table.oid AND target.attnum=keys.target_num
 WHERE con.contype='f' AND cardinality(con.conkey)=1 AND source_schema.nspname=$1 AND source_table.relname=$2 ORDER BY con.conname, source.attnum
			`, schema, name)
			if queryFKErr != nil {
				return queryFKErr
			}
			{
				for fkRows.Next() {
					var columnName, foreignSchema, foreignTable, targetColumn string
					if err := fkRows.Scan(&columnName, &foreignSchema, &foreignTable, &targetColumn); err != nil {
						fkRows.Close()
						return err
					}
					{
						relationInfo := RelationInfo{
							RelationName:  foreignTable,
							ForeignSchema: foreignSchema,
							ForeignTable:  foreignTable,
							LocalColumn:   columnName,
							TargetColumn:  targetColumn,
							IsArray:       false,
						}
						addRelation(tableInfo, foreignTable, relationInfo)
						// Also index by singular relation alias (e.g. author_id -> author)
						trimmedColumnName := strings.TrimSuffix(columnName, "_id")
						if trimmedColumnName != "" && trimmedColumnName != foreignTable {
							addRelation(tableInfo, trimmedColumnName, relationInfo)
						}
					}
				}
				fkRows.Close()
				if err := fkRows.Err(); err != nil {
					return err
				}
			}

			tableKey := fmt.Sprintf("%s.%s", schema, name)
			newTables[tableKey] = tableInfo
		}
	}

	// Snapshot forward edges before adding reverse relations to avoid traversing
	// newly added edges or choosing an arbitrary join for ambiguous foreign keys.
	forward := make(map[*TableInfo][]RelationInfo)
	for _, table := range newTables {
		for _, relation := range table.ForeignKeys {
			forward[table] = append(forward[table], relation)
		}
	}
	for childTable, relations := range forward {
		for _, foreignKey := range relations {
			targetSchema := foreignKey.ForeignSchema
			parentKey := fmt.Sprintf("%s.%s", targetSchema, foreignKey.ForeignTable)
			if parentTableInfo, exists := newTables[parentKey]; exists {
				{
					addRelation(parentTableInfo, childTable.Name, RelationInfo{
						RelationName:  childTable.Name,
						ForeignSchema: childTable.Schema,
						ForeignTable:  childTable.Name,
						LocalColumn:   foreignKey.TargetColumn,
						TargetColumn:  foreignKey.LocalColumn,
						IsArray:       true,
					})
				}
			}
		}
	}

	schemaInspector.rwMutex.Lock()
	schemaInspector.tables = newTables
	schemaInspector.rwMutex.Unlock()

	if data, err := json.Marshal(newTables); err == nil {
		_ = schemaInspector.kernel.KVStore().Set(ctx, "cache:schema:catalog", string(data), catalogTTL)
	}
	return nil
}
