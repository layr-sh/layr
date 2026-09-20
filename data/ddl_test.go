package data

import (
	"context"
	"testing"
)

func TestDataDDLValidationAndSanitizationUnit(t *testing.T) {
	// 1. IsProtectedSchema
	protectedList := []string{
		"system",
		"auth",
		"data",
		"storage",
		"scheduler",
		"notification",
		"analytics",
		"console",
		"core",
		"layr_auth",
		"layr_storage",
		"layr_scheduler",
		"layr_notification",
		"layr_analytics",
		"layr_console",
		"information_schema",
		"pg_catalog",
	}
	for _, schemaName := range protectedList {
		if !IsProtectedSchema(schemaName) {
			t.Fatalf("expected schema %q to be protected", schemaName)
		}
	}

	unprotectedList := []string{"public", "app", "store", "custom", "tenant1"}
	for _, schemaName := range unprotectedList {
		if IsProtectedSchema(schemaName) {
			t.Fatalf("expected schema %q to not be protected", schemaName)
		}
	}

	// 2. sanitizeDefaultValue
	validDefaults := []struct {
		input    string
		expected string
	}{
		{"''", "''"},
		{"null", "null"},
		{"NULL", "NULL"},
		{"42", "42"},
		{"3.1415", "3.1415"},
		{"true", "true"},
		{"FALSE", "FALSE"},
		{"now()", "now()"},
		{"uuidv7()", "uuidv7()"},
		{"clock_timestamp()", "clock_timestamp()"},
		{"'hello'", "'hello'"},
		{"'it''s'", "'it''s'"},
	}

	for _, testCase := range validDefaults {
		t.Run(testCase.input, func(t *testing.T) {
			actual, err := sanitizeDefaultValue(testCase.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", testCase.input, err)
			}
			if actual != testCase.expected {
				t.Fatalf("for input %q, expected %q, got %q", testCase.input, testCase.expected, actual)
			}
		})
	}

	// Invalid default value with injection / semicolon
	if _, err := sanitizeDefaultValue("42; DROP TABLE users;"); err == nil {
		t.Fatal("expected error on semicolon in sanitizeDefaultValue")
	}
	if _, err := sanitizeDefaultValue("   "); err == nil {
		t.Fatal("expected error on empty string in sanitizeDefaultValue")
	}

	// 3. sanitizeType
	if sanitizeType("VARCHAR(100)") != "varchar(100)" {
		t.Fatalf("unexpected sanitizeType: %s", sanitizeType("VARCHAR(100)"))
	}
	if sanitizeType("NUMERIC(10,2)") != "numeric(10,2)" {
		t.Fatalf("unexpected sanitizeType: %s", sanitizeType("NUMERIC(10,2)"))
	}
	if sanitizeType("INT; DROP TABLE") != "text" {
		t.Fatalf("expected fallback to text on invalid characters in type, got %s", sanitizeType("INT; DROP TABLE"))
	}

	// 4. sanitizeAction
	if sanitizeAction("CASCADE") != "CASCADE" || sanitizeAction("SET NULL") != "SET NULL" || sanitizeAction("RESTRICT") != "RESTRICT" {
		t.Fatal("unexpected sanitizeAction mapping")
	}
	if sanitizeAction("UNKNOWN") != "NO ACTION" {
		t.Fatalf("expected NO ACTION fallback, got %s", sanitizeAction("UNKNOWN"))
	}

	// 5. Protected schema validation in DDLEngine methods (pure in-memory checks before query execution)
	ctx := context.Background()
	ddlEngine := NewDDLEngine(nil)

	// CreateTable on protected schema
	err := ddlEngine.CreateTable(ctx, CreateTableInput{
		Schema: "core",
		Name:   "forbidden_table",
	})
	if err == nil || err.Error() != "cannot create tables in protected schema 'core'" {
		t.Fatalf("expected protected schema error on CreateTable, got: %v", err)
	}

	// AddColumn on protected schema
	err = ddlEngine.AddColumn(ctx, "layr_auth", "users", Column{Name: "new_column", Type: "text"})
	if err == nil || err.Error() != "cannot alter tables in protected schema 'layr_auth'" {
		t.Fatalf("expected protected schema error on AddColumn, got: %v", err)
	}

	// AlterColumn on protected schema
	err = ddlEngine.AlterColumn(ctx, "layr_auth", "users", "email", UpdateColumnInput{})
	if err == nil || err.Error() != "cannot alter tables in protected schema 'layr_auth'" {
		t.Fatalf("expected protected schema error on AlterColumn, got: %v", err)
	}

	// DropColumn on protected schema
	err = ddlEngine.DropColumn(ctx, "layr_auth", "users", "email", false)
	if err == nil || err.Error() != "cannot alter tables in protected schema 'layr_auth'" {
		t.Fatalf("expected protected schema error on DropColumn, got: %v", err)
	}

	// DropTable on protected schema
	err = ddlEngine.DropTable(ctx, "layr_storage", "objects", false)
	if err == nil || err.Error() != "cannot drop tables in protected schema 'layr_storage'" {
		t.Fatalf("expected protected schema error on DropTable, got: %v", err)
	}

	// TruncateTable on protected schema
	err = ddlEngine.TruncateTable(ctx, "layr_storage", "objects", false)
	if err == nil || err.Error() != "cannot truncate tables in protected schema 'layr_storage'" {
		t.Fatalf("expected protected schema error on TruncateTable, got: %v", err)
	}

	// ListPolicies on protected schema
	_, err = ddlEngine.ListPolicies(ctx, "layr_console", "users")
	if err == nil || err.Error() != "cannot view policies in protected schema 'layr_console'" {
		t.Fatalf("expected protected schema error on ListPolicies, got: %v", err)
	}

	// CreatePolicy on protected schema
	err = ddlEngine.CreatePolicy(ctx, "layr_analytics", "events", CreatePolicyInput{Name: "p1"})
	if err == nil || err.Error() != "cannot manage policies in protected schema 'layr_analytics'" {
		t.Fatalf("expected protected schema error on CreatePolicy, got: %v", err)
	}

	// DropPolicy on protected schema
	err = ddlEngine.DropPolicy(ctx, "layr_scheduler", "jobs", "p1")
	if err == nil || err.Error() != "cannot manage policies in protected schema 'layr_scheduler'" {
		t.Fatalf("expected protected schema error on DropPolicy, got: %v", err)
	}

	// EnableRLS on protected schema
	err = ddlEngine.EnableRLS(ctx, "layr_notification", "messages")
	if err == nil || err.Error() != "cannot alter table in protected schema 'layr_notification'" {
		t.Fatalf("expected protected schema error on EnableRLS, got: %v", err)
	}

	// DisableRLS on protected schema
	err = ddlEngine.DisableRLS(ctx, "information_schema", "tables")
	if err == nil || err.Error() != "cannot alter table in protected schema 'information_schema'" {
		t.Fatalf("expected protected schema error on DisableRLS, got: %v", err)
	}

	// ForceRLS on protected schema
	err = ddlEngine.ForceRLS(ctx, "pg_catalog", "tables", true)
	if err == nil || err.Error() != "cannot alter table in protected schema 'pg_catalog'" {
		t.Fatalf("expected protected schema error on ForceRLS, got: %v", err)
	}

	// Invalid identifier validation
	err = ddlEngine.CreateTable(ctx, CreateTableInput{
		Schema: "public",
		Name:   "invalid table name with spaces!",
	})
	if err == nil {
		t.Fatal("expected error on invalid table name")
	}

	err = ddlEngine.CreateTable(ctx, CreateTableInput{
		Schema: "public",
		Name:   "valid_table",
		Columns: []Column{
			{Name: "invalid column!", Type: "text"},
		},
	})
	if err == nil {
		t.Fatal("expected error on invalid column name")
	}

	// AddColumn invalid identifier
	err = ddlEngine.AddColumn(ctx, "public", "invalid table!", Column{Name: "column1", Type: "text"})
	if err == nil {
		t.Fatal("expected error on invalid table name in AddColumn")
	}

	// DropTable invalid identifier
	err = ddlEngine.DropTable(ctx, "public", "invalid table!", false)
	if err == nil {
		t.Fatal("expected error on invalid table name in DropTable")
	}
}
