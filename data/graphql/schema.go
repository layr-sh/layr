// Package graphql provides a single-shot GraphQL engine with schema introspection and caching.
package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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
			if err := rows.Scan(&name); err == nil {
				tableNames = append(tableNames, name)
			}
		}
		rows.Close()

		for _, name := range tableNames {
			tableInfo := &TableInfo{
				Schema:      schema,
				Name:        name,
				PrimaryKey:  "id",
				Columns:     make(map[string]ColumnInfo),
				ForeignKeys: make(map[string]RelationInfo),
			}

			// Fetch columns
			columnRows, queryColumnsErr := schemaInspector.kernel.DB().Query(ctx, `
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = $1 AND table_name = $2
			`, schema, name)
			if queryColumnsErr == nil {
				for columnRows.Next() {
					var columnName, dataType, isNullable string
					if err := columnRows.Scan(&columnName, &dataType, &isNullable); err == nil {
						tableInfo.Columns[columnName] = ColumnInfo{
							Name:       columnName,
							DataType:   dataType,
							IsNullable: isNullable == "YES",
						}
					}
				}
				columnRows.Close()
			}

			// Fetch primary key
			var pkName string
			_ = schemaInspector.kernel.DB().QueryRow(ctx, `
				SELECT kcu.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				  AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
				  AND tc.table_schema = $1
				  AND tc.table_name = $2
				LIMIT 1
			`, schema, name).Scan(&pkName)
			if pkName != "" {
				tableInfo.PrimaryKey = pkName
				if columnInfo, ok := tableInfo.Columns[pkName]; ok {
					columnInfo.IsPrimaryKey = true
					tableInfo.Columns[pkName] = columnInfo
				}
			}

			// Fetch foreign keys (including cross-schema foreign keys)
			fkRows, queryFKErr := schemaInspector.kernel.DB().Query(ctx, `
				SELECT
					kcu.column_name,
					ccu.table_schema AS foreign_table_schema,
					ccu.table_name AS foreign_table_name,
					ccu.column_name AS foreign_column_name
				FROM information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				  AND tc.table_schema = kcu.table_schema
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				WHERE tc.constraint_type = 'FOREIGN KEY'
				  AND tc.table_schema = $1
				  AND tc.table_name = $2
			`, schema, name)
			if queryFKErr == nil {
				for fkRows.Next() {
					var columnName, foreignSchema, foreignTable, targetColumn string
					if err := fkRows.Scan(&columnName, &foreignSchema, &foreignTable, &targetColumn); err == nil {
						relationInfo := RelationInfo{
							RelationName:  foreignTable,
							ForeignSchema: foreignSchema,
							ForeignTable:  foreignTable,
							LocalColumn:   columnName,
							TargetColumn:  targetColumn,
							IsArray:       false,
						}
						tableInfo.ForeignKeys[foreignTable] = relationInfo
						// Also index by singular relation alias (e.g. author_id -> author)
						trimmedColumnName := strings.TrimSuffix(columnName, "_id")
						if trimmedColumnName != "" && trimmedColumnName != foreignTable {
							tableInfo.ForeignKeys[trimmedColumnName] = relationInfo
						}
					}
				}
				fkRows.Close()
			}

			tableKey := fmt.Sprintf("%s.%s", schema, name)
			newTables[tableKey] = tableInfo
		}
	}

	// Second pass: Populate reverse (one-to-many) relations
	for _, childTable := range newTables {
		for _, foreignKey := range childTable.ForeignKeys {
			targetSchema := foreignKey.ForeignSchema
			parentKey := fmt.Sprintf("%s.%s", targetSchema, foreignKey.ForeignTable)
			if parentTableInfo, exists := newTables[parentKey]; exists {
				if _, alreadySet := parentTableInfo.ForeignKeys[childTable.Name]; !alreadySet {
					parentTableInfo.ForeignKeys[childTable.Name] = RelationInfo{
						RelationName:  childTable.Name,
						ForeignSchema: childTable.Schema,
						ForeignTable:  childTable.Name,
						LocalColumn:   foreignKey.TargetColumn,
						TargetColumn:  foreignKey.LocalColumn,
						IsArray:       true,
					}
				}
			}
		}
	}

	schemaInspector.rwMutex.Lock()
	schemaInspector.tables = newTables
	schemaInspector.rwMutex.Unlock()

	if data, err := json.Marshal(newTables); err == nil {
		_ = schemaInspector.kernel.KVStore().Set(ctx, "cache:schema:catalog", string(data), schemaInspector.catalogTTL)
	}
	return nil
}
