package filestorage

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerBucketUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	privilegedAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageBucketWrite,
		},
	}
	privilegedCtx := core.WithAuthContext(context.Background(), privilegedAuthContext)

	readOnlyAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFileStorageBucketRead,
		},
	}
	readOnlyCtx := core.WithAuthContext(context.Background(), readOnlyAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(context.Background(), noScopeAuthContext)

	t.Run("handle list buckets unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBuckets(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}
	})

	t.Run("handle create bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		readOnlyRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"test"}`)))
		readOnlyResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(readOnlyResponseRecorder, readOnlyRequest)
		if readOnlyResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for insufficient scope, got %d", readOnlyResponseRecorder.Code)
		}

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(badJSONResponseRecorder, badJSONRequest)
		if badJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for bad JSON, got %d", badJSONResponseRecorder.Code)
		}

		// Empty name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"   "}`)))
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}

		// Invalid bucket name (too short) -> 400
		invalidNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"ab"}`)))
		invalidNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(invalidNameResponseRecorder, invalidNameRequest)
		if invalidNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for short bucket name, got %d", invalidNameResponseRecorder.Code)
		}

		// Invalid bucket name (invalid characters) -> 400
		invalidCharsNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"Invalid_Bucket!"}`)))
		invalidCharsNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(invalidCharsNameResponseRecorder, invalidCharsNameRequest)
		if invalidCharsNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid characters bucket name, got %d", invalidCharsNameResponseRecorder.Code)
		}

		// Invalid backend -> 400
		badBackendRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(`{"name":"valid-name","backend":"unsupported-backend"}`)))
		badBackendResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateBucket(badBackendResponseRecorder, badBackendRequest)
		if badBackendResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsupported backend, got %d", badBackendResponseRecorder.Code)
		}
	})

	t.Run("handle get bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets/test-bucket", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodGet, "/v1/_/file-storage/buckets/", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})

	t.Run("handle update bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPatch, "/v1/_/file-storage/buckets/test-bucket", bytes.NewReader([]byte(`{}`)))
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/", bytes.NewReader([]byte(`{}`)))
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}

		// Invalid JSON -> 400
		badJSONRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodPatch, "/v1/_/file-storage/buckets/test-bucket", bytes.NewReader([]byte("{invalid-json")))
		badJSONRequest.SetPathValue("bucket", "test-bucket")
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateBucket(badJSONResponseRecorder, badJSONRequest)
		if badJSONResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid JSON, got %d", badJSONResponseRecorder.Code)
		}
	})

	t.Run("handle delete bucket unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/file-storage/buckets/test-bucket", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteBucket(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(privilegedCtx, http.MethodDelete, "/v1/_/file-storage/buckets/", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteBucket(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})

	t.Run("handle list bucket objects unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/file-storage/buckets/test-bucket/objects", nil)
		noScopeRequest.SetPathValue("bucket", "test-bucket")
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBucketObjects(noScopeResponseRecorder, noScopeRequest)
		if noScopeResponseRecorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", noScopeResponseRecorder.Code)
		}

		// Empty bucket name -> 400
		emptyNameRequest := httptest.NewRequestWithContext(readOnlyCtx, http.MethodGet, "/v1/_/file-storage/buckets//objects", nil)
		emptyNameRequest.SetPathValue("bucket", "")
		emptyNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListBucketObjects(emptyNameResponseRecorder, emptyNameRequest)
		if emptyNameResponseRecorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty bucket name, got %d", emptyNameResponseRecorder.Code)
		}
	})
}

func TestFilestorageControlPlaneBucketDatabaseErrorsUnit(t *testing.T) {
	for _, test := range []struct {
		name, method, suffix, body, query string
		result                            []pgproto3.BackendMessage
		status                            int
		want                              string
	}{
		{"update write", http.MethodPatch, "", `{}`, "UPDATE file_storage.buckets", postgresFailure(), 500, "Failed to update bucket"},
		{"delete lookup", http.MethodDelete, "", "", "FOR UPDATE", postgresFailure(), 500, "Failed to load bucket"},
		{"delete invalid s3 config", http.MethodDelete, "", "", "FOR UPDATE", postgresResult([]uint32{pgtype.UUIDOID, pgtype.TextOID, pgtype.TextOID, pgtype.JSONBOID}, []string{fixtureBucketID, "test-bucket", "s3", `[]`}), 500, "Invalid S3 bucket configuration"},
		{"delete list query", http.MethodDelete, "", "", "SELECT object_key", postgresFailure(), 500, "Failed to list bucket objects"},
		{"delete list stream", http.MethodDelete, "", "", "SELECT object_key", postgresStreamFailure(), 500, "Failed to read bucket objects"},
		{"delete no returned row", http.MethodDelete, "", "", "DELETE FROM file_storage.buckets", postgresResult([]uint32{pgtype.UUIDOID}), 404, "not found"},
		{"delete statement", http.MethodDelete, "", "", "DELETE FROM file_storage.buckets", postgresFailure(), 500, "injected PostgreSQL failure"},
		{"delete commit", http.MethodDelete, "", "", "commit", postgresFailure(), 500, "Failed to delete bucket"},
		{"list count", http.MethodGet, "/objects", "", "SELECT COUNT(*)", postgresFailure(), 500, "Failed to count objects"},
		{"list query", http.MethodGet, "/objects", "", "LIMIT", postgresFailure(), 500, "Failed to list objects"},
		{"list scan", http.MethodGet, "/objects", "", "LIMIT", fixtureObjectResult("invalid uuid"), 500, "Failed to read object"},
		{"list stream", http.MethodGet, "/objects", "", "LIMIT", postgresStreamFailure(), 500, "Failed to read objects"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kernel := postgresFaultKernel(t.Context(), t, func(sql string) []pgproto3.BackendMessage {
				switch {
				case strings.Contains(sql, test.query):
					return test.result
				case strings.Contains(sql, "FOR UPDATE"):
					return postgresResult([]uint32{pgtype.UUIDOID, pgtype.TextOID, pgtype.TextOID, pgtype.JSONBOID}, []string{fixtureBucketID, "test-bucket", "s3", `{}`})
				case strings.Contains(sql, "SELECT object_key"):
					return postgresResult([]uint32{pgtype.TextOID})
				case strings.Contains(sql, "SELECT id FROM file_storage.buckets"):
					return postgresResult([]uint32{pgtype.UUIDOID}, []string{fixtureBucketID})
				case strings.Contains(sql, "SELECT COUNT(*)"):
					return postgresResult([]uint32{pgtype.Int8OID}, []string{"1"})
				case strings.Contains(sql, "file_storage.buckets"):
					return fixtureBucketResult("database", "{}", "4")
				}
				return nil
			})
			controlPlaneHandler := NewService(kernel).ControlPlaneHandler()
			request := storageTestRequest(test.method, "/v1/_/file-storage/buckets/test-bucket"+test.suffix, test.body, core.ScopeFileStorageBucketRead+" "+core.ScopeFileStorageBucketWrite)
			responseRecorder := httptest.NewRecorder()
			switch test.method {
			case http.MethodPatch:
				controlPlaneHandler.handleUpdateBucket(responseRecorder, request)
			case http.MethodDelete:
				controlPlaneHandler.handleDeleteBucket(responseRecorder, request)
			case http.MethodGet:
				controlPlaneHandler.handleListBucketObjects(responseRecorder, request)
			}
			if responseRecorder.Code != test.status || !strings.Contains(responseRecorder.Body.String(), test.want) {
				t.Fatalf("expected %d with %q, got %d: %s", test.status, test.want, responseRecorder.Code, responseRecorder.Body)
			}
		})
	}
}

func TestFilestorageControlPlaneBucketEncryptionFailureUnit(t *testing.T) {
	cryptoKeyManager, err := func() (*core.CryptoKeyManager, error) {
		reader := rand.Reader
		defer func() { rand.Reader = reader }()
		rand.Reader = iotest.ErrReader(errors.New("nonce source failed"))
		return core.NewCryptoKeyManager(core.TestMasterEncryptionKeyHex)
	}()
	if err != nil {
		t.Fatal(err)
	}
	dbKernel := postgresFaultKernel(t.Context(), t, func(sql string) []pgproto3.BackendMessage {
		if strings.Contains(sql, "FROM file_storage.buckets") {
			return fixtureBucketResult("s3", "{}", "4")
		}
		return nil
	})
	kernel := core.NewTestKernel(dbKernel.DB(), core.WithCryptoKeyManager(cryptoKeyManager))
	controlPlaneHandler := NewService(kernel).ControlPlaneHandler()
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			request := storageTestRequest(method, "/v1/_/file-storage/buckets", `{"name":"test-bucket","backend":"s3","backend_config":{"secret_access_key":"secret"}}`, core.ScopeFileStorageBucketWrite)
			responseRecorder := httptest.NewRecorder()
			if method == http.MethodPost {
				controlPlaneHandler.handleCreateBucket(responseRecorder, request)
			} else {
				controlPlaneHandler.handleUpdateBucket(responseRecorder, request)
			}
			if responseRecorder.Code != http.StatusInternalServerError || !strings.Contains(responseRecorder.Body.String(), "Failed to encrypt S3 credentials") || strings.Contains(responseRecorder.Body.String(), "nonce source failed") {
				t.Fatalf("expected encryption failure, got %d: %s", responseRecorder.Code, responseRecorder.Body)
			}
		})
	}
}
