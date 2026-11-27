package filestorage

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"layr.sh/core"
)

func createTestServiceAccountWithS3Credentials(
	t *testing.T,
	kernel *core.Kernel,
	name string,
	scopes []string,
) (uuid.UUID, string, string) {
	t.Helper()
	ctx := context.Background()

	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name:   name,
		Scopes: scopes,
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	parsedID, err := uuid.Parse(serviceAccount.ID)
	if err != nil {
		t.Fatalf("failed to parse service account id: %v", err)
	}

	accessKeyID := "LFS" + strings.ToUpper(strings.ReplaceAll(uuid.NewV7().String(), "-", ""))[:17]
	secretAccessKey := serviceAccount.SecretKey

	encryptedSecretKey, err := kernel.CryptoKeyManager().EncryptField([]byte(secretAccessKey))
	if err != nil {
		t.Fatalf("failed to encrypt s3 secret key: %v", err)
	}

	const insertCredentialSQL = `
		INSERT INTO file_storage.s3_credentials (
			service_account_id, access_key_id, encrypted_secret_key
		) VALUES ($1, $2, $3)
	`
	_, err = kernel.DB().Exec(ctx, insertCredentialSQL, parsedID, accessKeyID, encryptedSecretKey)
	if err != nil {
		t.Fatalf("failed to insert s3 credential: %v", err)
	}

	return parsedID, accessKeyID, secretAccessKey
}

// These tests use a real pgx pool and PostgreSQL wire messages so callers see
// transport, decoding, and transaction errors through the existing kernel DB.
func postgresFaultKernel(ctx context.Context, t *testing.T, reply func(string) []pgproto3.BackendMessage) *core.Kernel {
	t.Helper()
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

const (
	fixtureBucketID  = "01900000-0000-7000-8000-000000000001"
	fixtureObjectID  = "01900000-0000-7000-8000-000000000002"
	fixtureTimestamp = "2025-01-01 00:00:00+00"
)

func fixtureBucketResult(backend, mimeTypes, size string) []pgproto3.BackendMessage {
	return postgresResult([]uint32{pgtype.UUIDOID, pgtype.TextOID, pgtype.BoolOID, pgtype.TextOID, pgtype.JSONBOID, pgtype.TextArrayOID, pgtype.Int8OID, pgtype.TimestamptzOID, pgtype.TimestamptzOID},
		[]string{fixtureBucketID, "test-bucket", "f", backend, "{}", mimeTypes, size, fixtureTimestamp, fixtureTimestamp})
}

func fixtureObjectResult(id string) []pgproto3.BackendMessage {
	return postgresResult([]uint32{pgtype.UUIDOID, pgtype.UUIDOID, pgtype.TextOID, pgtype.TextOID, pgtype.Int8OID, pgtype.TextOID, pgtype.JSONBOID, pgtype.TimestamptzOID, pgtype.TimestamptzOID},
		[]string{id, fixtureBucketID, "test.txt", "text/plain", "4", "checksum", "{}", fixtureTimestamp, fixtureTimestamp})
}

func storageTestRequest(method, url, body, scope string) *http.Request {
	ctx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "storage-test-account", JWT: core.JWTClaims{Role: "service_role", Scope: scope},
	})
	request := httptest.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	request.SetPathValue("bucket", "test-bucket")
	request.SetPathValue("key", "test.txt")
	return request
}
