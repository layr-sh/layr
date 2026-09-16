package data

import (
	"context"
	"testing"

	"layr.sh/data/common"
)

func TestDataDDLEngineLifecycleIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	ddlEngine := NewDDLEngine(db)

	// 1. IsProtectedSchema
	if !IsProtectedSchema("system") || !IsProtectedSchema("auth") || !IsProtectedSchema("information_schema") {
		t.Fatal("expected system schemas to be protected")
	}
	if IsProtectedSchema("public") || IsProtectedSchema("app") {
		t.Fatal("expected public and app schemas to not be protected")
	}

	// 2. Reject DDL on protected schema
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{
		Schema: "system",
		Name:   "hacked_table",
	}); err == nil {
		t.Fatal("expected error creating table in system")
	}

	// 3. Create Valid Table with UUIDv7, columns, and constraints
	defaultValue := "10"
	createTableRequest := CreateTableRequest{
		Schema: "public",
		Name:   "products",
		Columns: []ColumnDefinition{
			{Name: "id", IsPrimaryKey: true},
			{Name: "title", Type: "varchar(255)", IsNullable: false, IsUnique: true},
			{Name: "stock", Type: "int", IsNullable: false, DefaultValue: &defaultValue},
			{Name: "data", Type: "jsonb", IsNullable: true},
		},
	}
	if err := ddlEngine.CreateTable(ctx, createTableRequest); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Invalid table name
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{Schema: "public", Name: "invalid-name!"}); err == nil {
		t.Fatal("expected error on invalid table name")
	}
	// Invalid column name
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{Schema: "public", Name: "valid_tbl", Columns: []ColumnDefinition{{Name: "invalid-column!"}}}); err == nil {
		t.Fatal("expected error on invalid column name")
	}
	// Invalid PK name
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{Schema: "public", Name: "valid_tbl2", PrimaryKeyName: "invalid-primary-key!"}); err == nil {
		t.Fatal("expected error on invalid primary key name")
	}

	// 4. Create child table with Foreign Key
	orderCreateTableRequest := CreateTableRequest{
		Schema: "public",
		Name:   "orders",
		Columns: []ColumnDefinition{
			{Name: "product_id", Type: "uuid", IsNullable: false},
			{Name: "quantity", Type: "int", IsNullable: false},
		},
		ForeignKeys: []ForeignKeyDefinition{
			{
				Column:        "product_id",
				ForeignTable:  "products",
				ForeignColumn: "id",
				OnDelete:      "CASCADE",
				OnUpdate:      "RESTRICT",
			},
		},
	}
	if err := ddlEngine.CreateTable(ctx, orderCreateTableRequest); err != nil {
		t.Fatalf("CreateTable with FK failed: %v", err)
	}

	// Invalid FK identifier
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{
		Schema: "public",
		Name:   "invalid_fk_tbl",
		ForeignKeys: []ForeignKeyDefinition{
			{Column: "invalid-column!", ForeignTable: "products", ForeignColumn: "id"},
		},
	}); err == nil {
		t.Fatal("expected error on invalid FK identifier")
	}

	// 5. GetTable & ListTables
	tableSummary, getErr := ddlEngine.GetTable(ctx, "public", "products")
	if getErr != nil {
		t.Fatalf("GetTable failed: %v", getErr)
	}
	if tableSummary.Name != "products" || tableSummary.PrimaryKey != "id" || len(tableSummary.Columns) != 4 {
		t.Fatalf("unexpected summary: %+v", tableSummary)
	}

	// GetTable error on non-existent or protected
	if _, err := ddlEngine.GetTable(ctx, "public", "non_existent"); err == nil {
		t.Fatal("expected error on non-existent table")
	}
	if _, err := ddlEngine.GetTable(ctx, "system", "nodes"); err == nil {
		t.Fatal("expected error on protected table")
	}

	tables, listErr := ddlEngine.ListTables(ctx, []string{"public"})
	if listErr != nil || len(tables) < 2 {
		t.Fatalf("ListTables failed: %v, tables: %+v", listErr, tables)
	}

	// ListTables empty schemas fallback
	tblsDef, listDefErr := ddlEngine.ListTables(ctx, nil)
	if listDefErr != nil || len(tblsDef) < 2 {
		t.Fatalf("ListTables with default nil schemas failed: %v", listDefErr)
	}

	// 6. AddColumn
	newColumnDefinition := ColumnDefinition{
		Name:       "sku",
		Type:       "text",
		IsNullable: true,
	}
	if err := ddlEngine.AddColumn(ctx, "public", "products", newColumnDefinition); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}
	// AddColumn errors
	if err := ddlEngine.AddColumn(ctx, "auth", "users", newColumnDefinition); err == nil {
		t.Fatal("expected error adding column to protected schema")
	}
	if err := ddlEngine.AddColumn(ctx, "public", "invalid-table!", newColumnDefinition); err == nil {
		t.Fatal("expected error on invalid table name")
	}

	// 7. AlterColumn (Rename, Change Type, Nullability, Default)
	newName := "product_sku"
	newType := "varchar(100)"
	notNull := false
	newDef := "'UNKNOWN'"
	if err := ddlEngine.AlterColumn(ctx, "public", "products", "sku", AlterColumnRequest{
		NewName:      &newName,
		NewType:      &newType,
		IsNullable:   &notNull,
		DefaultValue: &newDef,
	}); err != nil {
		t.Fatalf("AlterColumn failed: %v", err)
	}

	// Alter column drop default and set not null
	emptyDef := ""
	isNotNull := true
	if err := ddlEngine.AlterColumn(ctx, "public", "products", "product_sku", AlterColumnRequest{
		DefaultValue: &emptyDef,
		IsNullable:   &isNotNull,
	}); err != nil {
		t.Fatalf("AlterColumn drop default failed: %v", err)
	}

	// AlterColumn errors
	if err := ddlEngine.AlterColumn(ctx, "auth", "users", "id", AlterColumnRequest{}); err == nil {
		t.Fatal("expected error on protected schema")
	}
	if err := ddlEngine.AlterColumn(ctx, "public", "invalid-table!", "id", AlterColumnRequest{}); err == nil {
		t.Fatal("expected error on invalid table name")
	}
	if err := ddlEngine.AlterColumn(ctx, "public", "products", "product_sku", AlterColumnRequest{}); err == nil {
		t.Fatal("expected error on empty alter column request")
	}
	badNewName := "invalid-new-name!"
	if err := ddlEngine.AlterColumn(ctx, "public", "products", "product_sku", AlterColumnRequest{NewName: &badNewName}); err == nil {
		t.Fatal("expected error on invalid new column name")
	}

	// 8. CreateIndex, ListIndexes, DropIndex
	createIndexRequest := CreateIndexRequest{
		IndexName: "idx_products_sku",
		Columns:   []string{"product_sku"},
		Type:      "btree",
		IsUnique:  false,
	}
	if err := ddlEngine.CreateIndex(ctx, "public", "products", createIndexRequest); err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Auto index name
	if err := ddlEngine.CreateIndex(ctx, "public", "products", CreateIndexRequest{Columns: []string{"title"}}); err != nil {
		t.Fatalf("CreateIndex with auto name failed: %v", err)
	}

	// CreateIndex errors
	if err := ddlEngine.CreateIndex(ctx, "auth", "nodes", createIndexRequest); err == nil {
		t.Fatal("expected error creating index on protected schema")
	}
	if err := ddlEngine.CreateIndex(ctx, "public", "invalid-table!", createIndexRequest); err == nil {
		t.Fatal("expected error on invalid table name")
	}
	if err := ddlEngine.CreateIndex(ctx, "public", "products", CreateIndexRequest{Columns: []string{"invalid-column!"}}); err == nil {
		t.Fatal("expected error on invalid column")
	}
	if err := ddlEngine.CreateIndex(ctx, "public", "products", CreateIndexRequest{Columns: []string{"product_sku"}, Type: "invalid-type"}); err == nil {
		t.Fatal("expected error on invalid index type")
	}

	indexes, listIdxErr := ddlEngine.ListIndexes(ctx, "public", "products")
	if listIdxErr != nil || len(indexes) == 0 {
		t.Fatalf("ListIndexes failed: %v", listIdxErr)
	}

	if _, err := ddlEngine.ListIndexes(ctx, "public", "invalid-table!"); err == nil {
		t.Fatal("expected error on invalid table name in ListIndexes")
	}

	if err := ddlEngine.DropIndex(ctx, "public", "idx_products_sku"); err != nil {
		t.Fatalf("DropIndex failed: %v", err)
	}
	// DropIndex errors
	if err := ddlEngine.DropIndex(ctx, "system", "idx"); err == nil {
		t.Fatal("expected error dropping index on protected schema")
	}
	if err := ddlEngine.DropIndex(ctx, "public", "invalid-idx!"); err == nil {
		t.Fatal("expected error on invalid index name")
	}

	// 9. DropColumn
	if err := ddlEngine.DropColumn(ctx, "public", "products", "product_sku", false); err != nil {
		t.Fatalf("DropColumn failed: %v", err)
	}
	// DropColumn errors
	if err := ddlEngine.DropColumn(ctx, "auth", "users", "id", false); err == nil {
		t.Fatal("expected error on protected schema")
	}
	if err := ddlEngine.DropColumn(ctx, "public", "invalid-table!", "id", false); err == nil {
		t.Fatal("expected error on invalid table name")
	}

	// 10. TruncateTable
	if err := ddlEngine.TruncateTable(ctx, "public", "products", true); err != nil {
		t.Fatalf("TruncateTable failed: %v", err)
	}
	// TruncateTable errors
	if err := ddlEngine.TruncateTable(ctx, "system", "nodes", false); err == nil {
		t.Fatal("expected error on protected schema")
	}
	if err := ddlEngine.TruncateTable(ctx, "public", "invalid-table!", false); err == nil {
		t.Fatal("expected error on invalid table name")
	}

	// 11. DropTable
	if err := ddlEngine.DropTable(ctx, "public", "orders", true); err != nil {
		t.Fatalf("DropTable orders failed: %v", err)
	}
	if err := ddlEngine.DropTable(ctx, "public", "products", true); err != nil {
		t.Fatalf("DropTable products failed: %v", err)
	}
	// DropTable errors
	if err := ddlEngine.DropTable(ctx, "system", "nodes", false); err == nil {
		t.Fatal("expected error on protected schema")
	}
	if err := ddlEngine.DropTable(ctx, "public", "invalid-table!", false); err == nil {
		t.Fatal("expected error on invalid table name")
	}

	// 13. Test empty schema defaults for all DDL operations
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{Name: "default_schema_tbl", Columns: []ColumnDefinition{{Name: "title", Type: "varchar(50)"}}}); err != nil {
		t.Fatalf("CreateTable with default schema failed: %v", err)
	}
	if err := ddlEngine.AddColumn(ctx, "", "default_schema_tbl", ColumnDefinition{Name: "num", Type: "numeric(10,2)"}); err != nil {
		t.Fatalf("AddColumn with default schema failed: %v", err)
	}
	columnRename := "num_renamed"
	if err := ddlEngine.AlterColumn(ctx, "", "default_schema_tbl", "num", AlterColumnRequest{NewName: &columnRename}); err != nil {
		t.Fatalf("AlterColumn with default schema failed: %v", err)
	}
	if err := ddlEngine.CreateIndex(ctx, "", "default_schema_tbl", CreateIndexRequest{Columns: []string{"num_renamed"}, IndexName: "idx_def_num"}); err != nil {
		t.Fatalf("CreateIndex with default schema failed: %v", err)
	}
	if err := ddlEngine.DropIndex(ctx, "", "idx_def_num"); err != nil {
		t.Fatalf("DropIndex with default schema failed: %v", err)
	}
	if err := ddlEngine.DropColumn(ctx, "", "default_schema_tbl", "num_renamed", false); err != nil {
		t.Fatalf("DropColumn with default schema failed: %v", err)
	}
	if err := ddlEngine.TruncateTable(ctx, "", "default_schema_tbl", false); err != nil {
		t.Fatalf("TruncateTable with default schema failed: %v", err)
	}
	if err := ddlEngine.DropTable(ctx, "", "default_schema_tbl", false); err != nil {
		t.Fatalf("DropTable with default schema failed: %v", err)
	}

	// 14. Helper tests: sanitizeAction, sanitizeType, isValidIdentifier
	if sanitizeAction("SET NULL") != "SET NULL" || sanitizeAction("SET DEFAULT") != "SET DEFAULT" || sanitizeAction("RESTRICT") != "RESTRICT" || sanitizeAction("unknown") != "NO ACTION" {
		t.Fatal("unexpected sanitizeAction results")
	}
	if sanitizeType("varchar(10)") != "varchar(10)" || sanitizeType("numeric(5,2)") != "numeric(5,2)" || sanitizeType("custom_unknown_type") != "text" {
		t.Fatal("unexpected sanitizeType results")
	}
	if common.IsValidIdentifier("") || common.IsValidIdentifier("1start_with_digit") || common.IsValidIdentifier("very_long_identifier_that_exceeds_sixty_three_characters_limit_in_postgresql_standard_length") {
		t.Fatal("expected common.IsValidIdentifier to reject invalid identifiers")
	}

	// 15. ExecuteSQL failed Exec branch (e.g. invalid syntax in DDL)
	if _, err := ddlEngine.ExecuteSQL(ctx, "CREATE TABLE invalid syntax error here;"); err == nil {
		t.Fatal("expected error on failed DDL execution")
	}

	// 16. Unique index creation and protected schemas in ListTables
	if _, err := ddlEngine.ListTables(ctx, []string{"system", "public"}); err != nil {
		t.Fatalf("ListTables with protected schema failed: %v", err)
	}

	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{
		Schema: "public",
		Name:   "unique_idx_tbl",
		Columns: []ColumnDefinition{
			{Name: "code", Type: "text", IsNullable: false},
		},
	}); err != nil {
		t.Fatalf("CreateTable unique_idx_tbl failed: %v", err)
	}

	if err := ddlEngine.CreateIndex(ctx, "public", "unique_idx_tbl", CreateIndexRequest{
		IndexName: "idx_unique_code",
		Columns:   []string{"code"},
		IsUnique:  true,
	}); err != nil {
		t.Fatalf("CreateIndex unique failed: %v", err)
	}

	// Failed DDL operations execution paths (e.g. duplicate column name or invalid type alter)
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{
		Schema: "public",
		Name:   "dup_cols_tbl",
		Columns: []ColumnDefinition{
			{Name: "title", Type: "text"},
			{Name: "title", Type: "text"},
		},
	}); err == nil {
		t.Fatal("expected error on duplicate columns in CreateTable")
	}

	if err := ddlEngine.DropColumn(ctx, "public", "unique_idx_tbl", "non_existent_column", true); err != nil {
		t.Fatalf("expected DropColumn IF EXISTS with cascade to succeed without error: %v", err)
	}

	badDef := "INVALID_SQL_EXPRESSION_HERE"
	if err := ddlEngine.AlterColumn(ctx, "public", "unique_idx_tbl", "code", AlterColumnRequest{DefaultValue: &badDef}); err == nil {
		t.Fatal("expected error on invalid default expression in AlterColumn")
	}

	_ = ddlEngine.DropTable(ctx, "public", "unique_idx_tbl", true)

	// 17. Error branches with canceled context
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		if _, err := ddlEngine.ListTables(canceledCtx, []string{"public"}); err == nil {
			t.Fatal("expected error on ListTables with canceled context")
		}
		if err := ddlEngine.DropTable(canceledCtx, "public", "test_tbl", false); err == nil {
			t.Fatal("expected error on DropTable with canceled context")
		}
		if err := ddlEngine.TruncateTable(canceledCtx, "public", "test_tbl", false); err == nil {
			t.Fatal("expected error on TruncateTable with canceled context")
		}
		newTypeName := "int"
		if err := ddlEngine.AlterColumn(canceledCtx, "public", "test_tbl", "column_name", AlterColumnRequest{NewType: &newTypeName}); err == nil {
			t.Fatal("expected error on AlterColumn with canceled context")
		}
		if err := ddlEngine.AddColumn(canceledCtx, "public", "test_tbl", ColumnDefinition{Name: "column_name", Type: "text"}); err == nil {
			t.Fatal("expected error on AddColumn with canceled context")
		}
		if err := ddlEngine.DropColumn(canceledCtx, "public", "test_tbl", "column_name", false); err == nil {
			t.Fatal("expected error on DropColumn with canceled context")
		}
		if _, err := ddlEngine.ListIndexes(canceledCtx, "public", "test_tbl"); err == nil {
			t.Fatal("expected error on ListIndexes with canceled context")
		}
		if err := ddlEngine.CreateIndex(canceledCtx, "public", "test_tbl", CreateIndexRequest{Columns: []string{"column_name"}}); err == nil {
			t.Fatal("expected error on CreateIndex with canceled context")
		}
		if err := ddlEngine.DropIndex(canceledCtx, "public", "test_idx"); err == nil {
			t.Fatal("expected error on DropIndex with canceled context")
		}
	}
	if _, err := ddlEngine.ListIndexes(ctx, "", "products"); err != nil {
		t.Fatalf("ListIndexes with empty schema failed: %v", err)
	}
	if err := ddlEngine.CreateIndex(ctx, "public", "products", CreateIndexRequest{IndexName: "valid_idx", Columns: []string{"invalid column!"}}); err == nil {
		t.Fatal("expected error on invalid column in CreateIndex")
	}
	if _, err := ddlEngine.ExecuteSQL(ctx, "   "); err == nil {
		t.Fatal("expected error on whitespace ExecuteSQL query")
	}

	// 18. RLS and Policy Management Tests
	if err := ddlEngine.CreateTable(ctx, CreateTableRequest{
		Schema: "public",
		Name:   "rls_test_tbl",
		Columns: []ColumnDefinition{
			{Name: "user_id", Type: "text", IsNullable: false},
			{Name: "org_id", Type: "text", IsNullable: false},
		},
	}); err != nil {
		t.Fatalf("CreateTable rls_test_tbl failed: %v", err)
	}

	// Enable RLS
	if err := ddlEngine.EnableRLS(ctx, "", "rls_test_tbl"); err != nil {
		t.Fatalf("EnableRLS with default schema failed: %v", err)
	}
	if err := ddlEngine.EnableRLS(ctx, "public", "rls_test_tbl"); err != nil {
		t.Fatalf("EnableRLS failed: %v", err)
	}
	// Verify GetTable reflects RLSEnabled
	rlsTableSummary, rlsGetErr := ddlEngine.GetTable(ctx, "public", "rls_test_tbl")
	if rlsGetErr != nil || !rlsTableSummary.RLSEnabled {
		t.Fatalf("expected RLSEnabled true, got %v, err: %v", rlsTableSummary.RLSEnabled, rlsGetErr)
	}

	// Force RLS
	if err := ddlEngine.ForceRLS(ctx, "public", "rls_test_tbl", true); err != nil {
		t.Fatalf("ForceRLS true failed: %v", err)
	}
	if err := ddlEngine.ForceRLS(ctx, "", "rls_test_tbl", false); err != nil {
		t.Fatalf("ForceRLS false with default schema failed: %v", err)
	}

	// Create Policy (Permissive, ALL, public, with USING and CHECK)
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{
		Name:            "policy_org_isolation",
		Command:         "ALL",
		Roles:           []string{"public"},
		Permissive:      "PERMISSIVE",
		UsingExpression: "org_id = 'org_123'",
		CheckExpression: "org_id = 'org_123'",
	}); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// Create Policy (Restrictive, SELECT, default roles)
	if err := ddlEngine.CreatePolicy(ctx, "", "rls_test_tbl", CreatePolicyRequest{
		Name:            "policy_user_select",
		Command:         "SELECT",
		Permissive:      "RESTRICTIVE",
		UsingExpression: "user_id = 'usr_123'",
	}); err != nil {
		t.Fatalf("CreatePolicy Restrictive SELECT failed: %v", err)
	}

	// Create Policy with specific role
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{
		Name:    "policy_authenticated_insert",
		Command: "INSERT",
		Roles:   []string{"layr"},
	}); err != nil {
		t.Fatalf("CreatePolicy with role failed: %v", err)
	}

	// List Policies
	policies, listPolErr := ddlEngine.ListPolicies(ctx, "public", "rls_test_tbl")
	if listPolErr != nil || len(policies) < 3 {
		t.Fatalf("ListPolicies failed: %v, count: %d", listPolErr, len(policies))
	}
	if _, err := ddlEngine.ListPolicies(ctx, "", "rls_test_tbl"); err != nil {
		t.Fatalf("ListPolicies empty schema failed: %v", err)
	}

	// Drop Policy
	if err := ddlEngine.DropPolicy(ctx, "public", "rls_test_tbl", "policy_org_isolation"); err != nil {
		t.Fatalf("DropPolicy failed: %v", err)
	}
	if err := ddlEngine.DropPolicy(ctx, "", "rls_test_tbl", "policy_user_select"); err != nil {
		t.Fatalf("DropPolicy with default schema failed: %v", err)
	}

	// Disable RLS
	if err := ddlEngine.DisableRLS(ctx, "", "rls_test_tbl"); err != nil {
		t.Fatalf("DisableRLS with default schema failed: %v", err)
	}

	// Error branches for RLS & Policies
	if err := ddlEngine.EnableRLS(ctx, "system", "nodes"); err == nil {
		t.Fatal("expected error on EnableRLS protected schema")
	}
	if err := ddlEngine.EnableRLS(ctx, "public", "invalid-table!"); err == nil {
		t.Fatal("expected error on EnableRLS invalid table")
	}

	if err := ddlEngine.DisableRLS(ctx, "system", "nodes"); err == nil {
		t.Fatal("expected error on DisableRLS protected schema")
	}
	if err := ddlEngine.DisableRLS(ctx, "public", "invalid-table!"); err == nil {
		t.Fatal("expected error on DisableRLS invalid table")
	}

	if err := ddlEngine.ForceRLS(ctx, "system", "nodes", true); err == nil {
		t.Fatal("expected error on ForceRLS protected schema")
	}
	if err := ddlEngine.ForceRLS(ctx, "public", "invalid-table!", true); err == nil {
		t.Fatal("expected error on ForceRLS invalid table")
	}

	if _, err := ddlEngine.ListPolicies(ctx, "system", "nodes"); err == nil {
		t.Fatal("expected error on ListPolicies protected schema")
	}
	if _, err := ddlEngine.ListPolicies(ctx, "public", "invalid-table!"); err == nil {
		t.Fatal("expected error on ListPolicies invalid table")
	}

	// Canceled context checks for RLS & Policies
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		if err := ddlEngine.EnableRLS(canceledCtx, "public", "rls_test_tbl"); err == nil {
			t.Fatal("expected error on EnableRLS with canceled context")
		}
		if err := ddlEngine.DisableRLS(canceledCtx, "public", "rls_test_tbl"); err == nil {
			t.Fatal("expected error on DisableRLS with canceled context")
		}
		if err := ddlEngine.ForceRLS(canceledCtx, "public", "rls_test_tbl", true); err == nil {
			t.Fatal("expected error on ForceRLS with canceled context")
		}
		if _, err := ddlEngine.ListPolicies(canceledCtx, "public", "rls_test_tbl"); err == nil {
			t.Fatal("expected error on ListPolicies with canceled context")
		}
		if err := ddlEngine.DropPolicy(canceledCtx, "public", "rls_test_tbl", "p"); err == nil {
			t.Fatal("expected error on DropPolicy with canceled context")
		}
	}

	// CreatePolicy error validations
	if err := ddlEngine.CreatePolicy(ctx, "system", "nodes", CreatePolicyRequest{Name: "p"}); err == nil {
		t.Fatal("expected error on CreatePolicy protected schema")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "invalid-table!", CreatePolicyRequest{Name: "p"}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid table")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "invalid-name!"}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid name")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", Command: "INVALID"}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid command")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", Permissive: "INVALID"}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid permissive")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", Roles: []string{"invalid-role!"}}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid role")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", UsingExpression: "INVALID SQL SYNTAX HERE;"}); err == nil {
		t.Fatal("expected error on CreatePolicy invalid SQL expression")
	}

	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", UsingExpression: "invalid syntax $$%"}); err == nil {
		t.Fatal("expected error on CreatePolicy with syntax error in UsingExpression")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", UsingExpression: "true; DROP TABLE"}); err == nil {
		t.Fatal("expected error on CreatePolicy with semicolon in UsingExpression")
	}
	if err := ddlEngine.CreatePolicy(ctx, "public", "rls_test_tbl", CreatePolicyRequest{Name: "p", CheckExpression: "true -- comment"}); err == nil {
		t.Fatal("expected error on CreatePolicy with comment in CheckExpression")
	}

	// AlterColumn bad default value
	badDefaultValue := "value; DROP TABLE"
	if err := ddlEngine.AlterColumn(ctx, "public", "rls_test_tbl", "id", AlterColumnRequest{DefaultValue: &badDefaultValue}); err == nil {
		t.Fatal("expected error on AlterColumn with bad default value")
	}

	// DropPolicy error validations
	if err := ddlEngine.DropPolicy(ctx, "system", "nodes", "p"); err == nil {
		t.Fatal("expected error on DropPolicy protected schema")
	}
	if err := ddlEngine.DropPolicy(ctx, "public", "invalid-table!", "p"); err == nil {
		t.Fatal("expected error on DropPolicy invalid table")
	}
	if err := ddlEngine.DropPolicy(ctx, "public", "rls_test_tbl", "invalid-policy!"); err == nil {
		t.Fatal("expected error on DropPolicy invalid policy name")
	}

	_ = ddlEngine.DropTable(ctx, "public", "rls_test_tbl", true)
}
