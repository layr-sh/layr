package graphql

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
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

// These tests use a real pgx pool and PostgreSQL wire messages so callers see
// transport, decoding, and transaction errors through the existing kernel DB.
func postgresFaultKernel(t *testing.T, reply func(string) []pgproto3.BackendMessage) *core.Kernel {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config, configErr := pgxpool.ParseConfig("postgres://test@localhost/test?sslmode=disable&default_query_exec_mode=simple_protocol")
	if configErr != nil {
		t.Fatal(configErr)
	}
	config.MaxConns = 1
	config.ConnConfig.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		clientConnection, connection := net.Pipe()
		go func() {
			defer func() { _ = connection.Close() }()
			backend := pgproto3.NewBackend(connection, connection)
			if _, err := backend.ReceiveStartupMessage(); err != nil {
				return
			}
			backend.Send(&pgproto3.AuthenticationOk{})
			backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "18.0"})
			backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
			backend.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
			backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := backend.Flush(); err != nil {
				return
			}
			for {
				frontendMessage, err := backend.Receive()
				if err != nil {
					return
				}
				switch query := frontendMessage.(type) {
				case *pgproto3.Terminate:
					return
				case *pgproto3.Query:
					messages := reply(query.String)
					if messages == nil {
						messages = []pgproto3.BackendMessage{&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")}}
					}
					for _, response := range messages {
						backend.Send(response)
					}
					backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
					if err := backend.Flush(); err != nil {
						return
					}
				default:
					return
				}
			}
		}()
		return clientConnection, nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return core.NewTestKernel(&core.DatabasePool{Pool: pool})
}

func postgresFields(oids ...uint32) *pgproto3.RowDescription {
	fields := make([]pgproto3.FieldDescription, len(oids))
	for index, oid := range oids {
		fields[index] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("field%d", index)), DataTypeOID: oid, DataTypeSize: -1}
	}
	return &pgproto3.RowDescription{Fields: fields}
}

func postgresResult(oids []uint32, rows ...[]string) []pgproto3.BackendMessage {
	messages := []pgproto3.BackendMessage{postgresFields(oids...)}
	for _, row := range rows {
		values := make([][]byte, len(row))
		for index, value := range row {
			if value != "<NULL>" {
				values[index] = []byte(value)
			}
		}
		messages = append(messages, &pgproto3.DataRow{Values: values})
	}
	return append(messages, &pgproto3.CommandComplete{CommandTag: []byte(fmt.Sprintf("SELECT %d", len(rows)))})
}

func postgresFailure() []pgproto3.BackendMessage {
	return []pgproto3.BackendMessage{&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: "injected PostgreSQL failure"}}
}

func postgresStreamFailure() []pgproto3.BackendMessage {
	return append([]pgproto3.BackendMessage{postgresFields(25)}, postgresFailure()...)
}

func TestGraphqlSchemaIntrospectionDatabaseErrorsUnit(t *testing.T) {
	queries := []struct {
		name, sql string
		columns   int
		row       []string
	}{
		{"tables", "FROM information_schema.tables", 1, []string{"items"}},
		{"columns", "FROM information_schema.columns", 3, []string{"id", "integer", "NO"}},
		{"primary key", "SELECT min(kcu.column_name", 1, []string{"id"}},
		{"foreign keys", "SELECT source.attname", 4, []string{"parent_id", "public", "parents", "id"}},
	}
	for _, target := range queries {
		for _, failure := range []string{"query", "scan", "stream"} {
			if (target.name == "tables" && failure == "query") || (target.name == "primary key" && failure != "query") {
				continue
			}
			t.Run(target.name+"/"+failure, func(t *testing.T) {
				kernel := postgresFaultKernel(t, func(sql string) []pgproto3.BackendMessage {
					for _, query := range queries {
						if !strings.Contains(sql, query.sql) {
							continue
						}
						oids := make([]uint32, query.columns)
						for i := range oids {
							oids[i] = 25 // text
						}
						if query.name == target.name {
							switch failure {
							case "query":
								return postgresFailure()
							case "scan":
								row := append([]string(nil), query.row...)
								row[0] = "<NULL>"
								return postgresResult(oids, row)
							case "stream":
								return postgresStreamFailure()
							}
						}
						return postgresResult(oids, query.row)
					}
					return nil
				})
				schemaIntrospector := NewSchemaIntrospector(kernel)
				err := schemaIntrospector.Introspect(context.Background(), []string{"public"})
				want := "injected PostgreSQL failure"
				if failure == "scan" {
					want = "cannot scan NULL"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("expected %s, got %v", want, err)
				}
				if len(schemaIntrospector.Tables()) != 0 {
					t.Fatal("incomplete catalog should not be published")
				}
			})
		}
	}
}
