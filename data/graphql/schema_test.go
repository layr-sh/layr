package graphql

import (
	"testing"
	"time"

	"layr.sh/core"
)

func TestGraphqlSchemaMetadataCacheUnit(t *testing.T) {
	// 1. NewSchemaIntrospector initializes empty tables map
	schemaIntrospector := NewSchemaIntrospector(core.NewTestKernel(nil))
	if schemaIntrospector == nil {
		t.Fatal("expected non-nil SchemaIntrospector")
	}
	if schemaIntrospector.tables == nil {
		t.Fatal("expected initialized tables map")
	}

	// 2. GetTable on missing table returns false
	_, exists := schemaIntrospector.GetTable("public", "missing_table")
	if exists {
		t.Fatal("expected false for missing table")
	}

	// 4. SetTable on registered table
	schemaIntrospector.SetTable(nil)
	schemaIntrospector.SetTable(&TableInfo{
		Schema:     "",
		Name:       "products",
		PrimaryKey: "id",
		Columns: map[string]ColumnInfo{
			"id": {
				Name:         "id",
				DataType:     "uuid",
				IsNullable:   false,
				IsPrimaryKey: true,
			},
			"title": {
				Name:       "title",
				DataType:   "text",
				IsNullable: false,
			},
		},
		ForeignKeys: map[string]RelationInfo{
			"reviews": {
				RelationName: "reviews",
				ForeignTable: "public.reviews",
				LocalColumn:  "id",
				TargetColumn: "product_id",
				IsArray:      true,
			},
		},
	})

	retrievedTableInfo, ok := schemaIntrospector.GetTable("public", "products")
	if !ok || retrievedTableInfo == nil {
		t.Fatal("expected to retrieve registered table public.products")
	}
	if retrievedTableInfo.PrimaryKey != "id" {
		t.Fatalf("expected primary key id, got %s", retrievedTableInfo.PrimaryKey)
	}
	if len(retrievedTableInfo.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(retrievedTableInfo.Columns))
	}
	if len(retrievedTableInfo.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key relation, got %d", len(retrievedTableInfo.ForeignKeys))
	}
	if retrievedTableInfo.ForeignKeys["reviews"].ForeignTable != "public.reviews" {
		t.Fatalf("expected foreign table public.reviews, got %s", retrievedTableInfo.ForeignKeys["reviews"].ForeignTable)
	}

	// 5. GetTable with empty schema defaults to public
	defaultSchemaTableInfo, ok := schemaIntrospector.GetTable("", "products")
	if !ok || defaultSchemaTableInfo == nil {
		t.Fatal("expected GetTable with empty schema to find public.products")
	}

	// 6. SetCatalogTTL and Tables snapshot
	schemaIntrospector.SetCatalogTTL(30 * time.Minute)
	allTables := schemaIntrospector.Tables()
	if len(allTables) != 1 {
		t.Fatalf("expected 1 table in snapshot, got %d", len(allTables))
	}
}
