package data

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgproto3"
	"layr.sh/core"
	"strings"
	"testing"
	"time"
)

func TestDataDDLValidationAndSanitizationUnit(t *testing.T) {
	// 1. IsProtectedSchema
	protectedList := []string{
		"system",
		"core",
		"auth",
		"data",
		"file_storage",
		"tasks",
		"image",
		"function",
		"console",
		"information_schema",
		"pg_catalog",
		"pg_toast",
	}
	for _, schemaName := range protectedList {
		t.Run(fmt.Sprintf("Protected_%s", schemaName), func(t *testing.T) {
			if !IsProtectedSchema(schemaName) {
				t.Fatalf("expected schema %q to be protected", schemaName)
			}
		})
	}

	unprotectedList := []string{"public", "app", "store", "custom", "tenant1"}
	for _, schemaName := range unprotectedList {
		t.Run(fmt.Sprintf("Unprotected_%s", schemaName), func(t *testing.T) {
			if IsProtectedSchema(schemaName) {
				t.Fatalf("expected schema %q to not be protected", schemaName)
			}
		})
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
	if sanitizeType("INT; DROP TABLE") != "" {
		t.Fatalf("expected invalid types to be rejected, got %s", sanitizeType("INT; DROP TABLE"))
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
	ddlEngine := NewDDLEngine(core.NewTestKernel(nil))

	// CreateTable on protected schema
	err := ddlEngine.CreateTable(ctx, CreateTableInput{
		Schema: "core",
		Name:   "forbidden_table",
	})
	if err == nil || err.Error() != "cannot create tables in protected schema 'core'" {
		t.Fatalf("expected protected schema error on CreateTable, got: %v", err)
	}

	// AddColumn on protected schema
	err = ddlEngine.AddColumn(ctx, "auth", "users", Column{Name: "new_column", Type: "text"})
	if err == nil || err.Error() != "cannot alter tables in protected schema 'auth'" {
		t.Fatalf("expected protected schema error on AddColumn, got: %v", err)
	}

	// AlterColumn on protected schema
	err = ddlEngine.AlterColumn(ctx, "auth", "users", "email", UpdateColumnInput{})
	if err == nil || err.Error() != "cannot alter tables in protected schema 'auth'" {
		t.Fatalf("expected protected schema error on AlterColumn, got: %v", err)
	}

	// DropColumn on protected schema
	err = ddlEngine.DropColumn(ctx, "auth", "users", "email", false)
	if err == nil || err.Error() != "cannot alter tables in protected schema 'auth'" {
		t.Fatalf("expected protected schema error on DropColumn, got: %v", err)
	}

	// DropTable on protected schema
	err = ddlEngine.DropTable(ctx, "file_storage", "objects", false)
	if err == nil || err.Error() != "cannot drop tables in protected schema 'file_storage'" {
		t.Fatalf("expected protected schema error on DropTable, got: %v", err)
	}

	// TruncateTable on protected schema
	err = ddlEngine.TruncateTable(ctx, "file_storage", "objects", false)
	if err == nil || err.Error() != "cannot truncate tables in protected schema 'file_storage'" {
		t.Fatalf("expected protected schema error on TruncateTable, got: %v", err)
	}

	// ListPolicies on protected schema
	_, err = ddlEngine.ListPolicies(ctx, "console", "users")
	if err == nil || err.Error() != "cannot view policies in protected schema 'console'" {
		t.Fatalf("expected protected schema error on ListPolicies, got: %v", err)
	}

	// CreatePolicy on protected schema
	err = ddlEngine.CreatePolicy(ctx, "file_storage", "objects", CreatePolicyInput{Name: "p1"})
	if err == nil || err.Error() != "cannot manage policies in protected schema 'file_storage'" {
		t.Fatalf("expected protected schema error on CreatePolicy, got: %v", err)
	}

	// DropPolicy on protected schema
	err = ddlEngine.DropPolicy(ctx, "tasks", "jobs", "p1")
	if err == nil || err.Error() != "cannot manage policies in protected schema 'tasks'" {
		t.Fatalf("expected protected schema error on DropPolicy, got: %v", err)
	}

	// EnableRLS on protected schema
	err = ddlEngine.EnableRLS(ctx, "auth", "users")
	if err == nil || err.Error() != "cannot alter table in protected schema 'auth'" {
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

func TestDataCatalogWireFailuresUnit(t *testing.T) {
	for _, testCase := range []struct{ stage, mode, operation string }{
		{"exists", "query", "get"}, {"primary", "query", "get"}, {"ddl columns", "query", "get"}, {"ddl columns", "scan", "get"}, {"ddl columns", "stream", "get"}, {"ddl foreign", "query", "get"}, {"ddl foreign", "scan", "get"}, {"ddl foreign", "stream", "get"}, {"indexes", "query", "get"}, {"indexes", "scan", "get"}, {"indexes", "stream", "get"}, {"stats", "query", "get"}, {"ddl tables", "scan", "list"}, {"ddl tables", "stream", "list"}, {"exists", "query", "list"},
		{"schema tables", "scan", "introspect"}, {"schema tables", "stream", "introspect"}, {"schema columns", "query", "introspect"}, {"schema columns", "scan", "introspect"}, {"schema columns", "stream", "introspect"}, {"primary", "query", "introspect"}, {"schema foreign", "query", "introspect"}, {"schema foreign", "scan", "introspect"}, {"schema foreign", "stream", "introspect"},
		{"schemas", "scan", "grant"}, {"schemas", "stream", "grant"},
	} {
		t.Run(testCase.operation+"/"+testCase.stage+"/"+testCase.mode, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			kernel := postgresFaultKernel(t, func(sql string) []pgproto3.BackendMessage {
				if catalogQueryStage(sql) == testCase.stage {
					select {
					case reached <- struct{}{}:
					default:
					}
					switch testCase.mode {
					case "scan":
						return postgresResult([]uint32{25, 25}, []string{"wrong", "shape"})
					case "stream":
						return postgresStreamFailure()
					default:
						return postgresFailure()
					}
				}
				return catalogQueryReply(sql)
			})
			service := NewService(kernel)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var err error
			switch testCase.operation {
			case "get":
				_, err = service.DDLEngine().GetTable(ctx, "public", "items")
			case "list":
				_, err = service.DDLEngine().ListTables(ctx, []string{"public"})
			case "introspect":
				err = service.GraphQLSchema().Introspect(ctx, []string{"public"})
			case "grant":
				err = service.grantQuerySchemas(ctx, []string{"public"})
			}
			if err == nil || len(reached) == 0 {
				t.Fatalf("database failure not exercised/propagated: reached=%v error=%v", (len(reached) > 0), err)
			}
		})
	}
}

func TestDataSQLWireFailuresUnit(t *testing.T) {
	for _, stage := range []string{"begin", "settings", "values", "commit", "acquire"} {
		t.Run(stage, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			kernel := postgresFaultKernel(t, func(sql string) []pgproto3.BackendMessage {
				statement := strings.ToLower(strings.TrimSpace(sql))
				if stage == "begin" && statement == "begin" || stage == "settings" && strings.HasPrefix(statement, "set local standard") || stage == "commit" && statement == "commit" {
					select {
					case reached <- struct{}{}:
					default:
					}
					return postgresFailure()
				}
				if strings.HasPrefix(statement, "select ") {
					if stage == "values" {
						select {
						case reached <- struct{}{}:
						default:
						}
						return postgresResult([]uint32{23}, []string{"invalid-integer"})
					}
					return postgresResult([]uint32{23}, []string{"1"})
				}
				return nil
			})
			if stage == "acquire" {
				kernel.DB().Close()
				select {
				case reached <- struct{}{}:
				default:
				}
			}
			if _, err := NewDDLEngine(kernel).ExecuteSQL(context.Background(), "SELECT 1"); err == nil || len(reached) == 0 {
				t.Fatalf("SQL fault not exercised: reached=%v error=%v", (len(reached) > 0), err)
			}
		})
	}
}
