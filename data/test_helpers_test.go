package data

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"layr.sh/core"
)

func dataAdminContext() context.Context {
	return core.WithAuthContext(context.Background(), core.AuthContext{ServiceAccountID: "data-test-admin", JWT: core.JWTClaims{Subject: "data-test-admin", Role: "service_role", Scope: "*"}})
}

// These tests use a real pgx pool and PostgreSQL wire messages so callers see
// transport, decoding, and transaction errors through the existing kernel DB.
func postgresFaultKernel(t *testing.T, reply func(string) []pgproto3.BackendMessage) *core.Kernel {
	t.Helper()
	listener, listenErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
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
		}
	}()
	db, err := core.NewDatabasePool(context.Background(), fmt.Sprintf("postgres://test@%s/test?sslmode=disable&default_query_exec_mode=simple_protocol", listener.Addr()), core.DatabasePoolOptions{MaxConns: 1, MinConns: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return core.NewTestKernel(db)
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

func catalogQueryStage(sql string) string {
	switch {
	case strings.Contains(sql, "SELECT EXISTS ("):
		return "exists"
	case strings.Contains(sql, "min(kcu.column_name"):
		return "primary"
	case strings.Contains(sql, "format_type(a.atttypid"):
		return "ddl columns"
	case strings.Contains(sql, "SELECT con.conname"):
		return "ddl foreign"
	case strings.Contains(sql, "AS index_name"):
		return "indexes"
	case strings.Contains(sql, "c.relrowsecurity"):
		return "stats"
	case strings.Contains(sql, "COALESCE(c.reltuples"):
		return "ddl tables"
	case strings.Contains(sql, "SELECT table_name"):
		return "schema tables"
	case strings.Contains(sql, "SELECT column_name, data_type"):
		return "schema columns"
	case strings.Contains(sql, "SELECT source.attname"):
		return "schema foreign"
	case strings.Contains(sql, "SELECT nspname"):
		return "schemas"
	}
	return ""
}

func catalogQueryReply(sql string) []pgproto3.BackendMessage {
	switch catalogQueryStage(sql) {
	case "exists":
		return postgresResult([]uint32{16}, []string{"t"})
	case "primary":
		return postgresResult([]uint32{25}, []string{"id"})
	case "ddl columns":
		return postgresResult([]uint32{25, 25, 25, 25, 16, 16}, []string{"id", "integer", "NO", "<NULL>", "t", "t"})
	case "ddl foreign":
		return postgresResult([]uint32{25, 25, 25, 25, 25, 25, 25}, []string{"fk", "parent_id", "public", "parents", "id", "NO ACTION", "NO ACTION"})
	case "indexes":
		return postgresResult([]uint32{25, 25, 16, 1009}, []string{"idx", "btree", "f", "{id}"})
	case "stats":
		return postgresResult([]uint32{16, 16, 20, 20}, []string{"f", "f", "1", "100"})
	case "ddl tables":
		return postgresResult([]uint32{25, 20, 20}, []string{"items", "1", "100"})
	case "schema tables":
		return postgresResult([]uint32{25}, []string{"items"})
	case "schema columns":
		return postgresResult([]uint32{25, 25, 25}, []string{"id", "integer", "NO"})
	case "schema foreign":
		return postgresResult([]uint32{25, 25, 25, 25}, []string{"parent_id", "public", "parents", "id"})
	case "schemas":
		return postgresResult([]uint32{25}, []string{"public"})
	}
	return nil
}

type unmarshalableIntCodec struct{ pgtype.Int4Codec }

func (unmarshalableIntCodec) DecodeValue(_ *pgtype.Map, _ uint32, _ int16, _ []byte) (any, error) {
	return func() {}, nil
}
