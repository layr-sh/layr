package s3sigv4

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

const testMasterEncryptionKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupTestDatabase(t *testing.T) (*core.DatabasePool, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, startErr := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr_s3sigv4_test"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if startErr != nil {
		t.Skipf("failed to start postgres container: %v", startErr)
		return nil, func() {}
	}

	databaseURL, connErr := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if connErr != nil {
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to get connection string: %v", connErr)
	}

	db, dbErr := core.NewDatabasePool(ctx, databaseURL)
	if dbErr != nil {
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to create database pool: %v", dbErr)
	}

	if migrateErr := db.RunMigrations(ctx, core.SystemDatabaseMigrations); migrateErr != nil {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to run system migrations: %v", migrateErr)
	}

	const createS3CredentialsTableSQL = `
		CREATE SCHEMA IF NOT EXISTS file_storage;
		CREATE TABLE IF NOT EXISTS file_storage.s3_credentials (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			service_account_id UUID NOT NULL REFERENCES core.service_accounts(id) ON DELETE CASCADE,
			access_key_id VARCHAR(128) NOT NULL UNIQUE,
			encrypted_secret_key TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`
	if _, createSchemaErr := db.Exec(ctx, createS3CredentialsTableSQL); createSchemaErr != nil {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to create s3_credentials table: %v", createSchemaErr)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
	}

	return db, cleanup
}

func signTestRequest(
	request *http.Request,
	accessKeyID string,
	secretAccessKey string,
	requestTime time.Time,
	isQueryAuth bool,
) {
	region := "us-east-1"
	service := "s3"
	dateStamp := requestTime.Format("20060102")
	amzDate := requestTime.Format("20060102T150405Z")
	credentialScope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	signedHeaders := "host;x-amz-date"

	if request.Host == "" {
		request.Host = "example.com"
	}
	request.Header.Set("Host", request.Host)

	var hashedPayload string
	if request.Body != nil {
		bodyBytes, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		hashedPayload = sha256Hex(bodyBytes)
	} else {
		hashedPayload = sha256Hex(nil)
	}

	if !isQueryAuth {
		request.Header.Set("X-Amz-Date", amzDate)
		canonicalURI := request.URL.EscapedPath()
		if canonicalURI == "" {
			canonicalURI = "/"
		}
		canonicalQuery := buildCanonicalQueryString(request.URL.Query(), false)
		canonicalHeaders := buildCanonicalHeaders(request, signedHeaders)
		canonicalRequest := strings.Join([]string{
			request.Method,
			canonicalURI,
			canonicalQuery,
			canonicalHeaders,
			signedHeaders,
			hashedPayload,
		}, "\n")

		stringToSign := strings.Join([]string{
			"AWS4-HMAC-SHA256",
			amzDate,
			credentialScope,
			sha256Hex([]byte(canonicalRequest)),
		}, "\n")

		kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(dateStamp))
		kRegion := hmacSHA256(kDate, []byte(region))
		kService := hmacSHA256(kRegion, []byte(service))
		kSigning := hmacSHA256(kService, []byte("aws4_request"))
		signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
			accessKeyID, credentialScope, signedHeaders, signature)
		request.Header.Set("Authorization", authHeader)
	} else {
		queryValues := request.URL.Query()
		queryValues.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
		queryValues.Set("X-Amz-Credential", accessKeyID+"/"+credentialScope)
		queryValues.Set("X-Amz-Date", amzDate)
		queryValues.Set("X-Amz-SignedHeaders", signedHeaders)
		request.URL.RawQuery = queryValues.Encode()

		canonicalURI := request.URL.EscapedPath()
		if canonicalURI == "" {
			canonicalURI = "/"
		}
		canonicalQuery := buildCanonicalQueryString(queryValues, true)
		canonicalHeaders := buildCanonicalHeaders(request, signedHeaders)
		canonicalRequest := strings.Join([]string{
			request.Method,
			canonicalURI,
			canonicalQuery,
			canonicalHeaders,
			signedHeaders,
			"UNSIGNED-PAYLOAD",
		}, "\n")

		stringToSign := strings.Join([]string{
			"AWS4-HMAC-SHA256",
			amzDate,
			credentialScope,
			sha256Hex([]byte(canonicalRequest)),
		}, "\n")

		kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(dateStamp))
		kRegion := hmacSHA256(kDate, []byte(region))
		kService := hmacSHA256(kRegion, []byte(service))
		kSigning := hmacSHA256(kService, []byte("aws4_request"))
		signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

		queryValues.Set("X-Amz-Signature", signature)
		request.URL.RawQuery = queryValues.Encode()
	}
}

func TestS3sigv4ValidationIntegration(t *testing.T) {
	db, cleanup := setupTestDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	cryptoKeyManager, cryptoErr := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if cryptoErr != nil {
		t.Fatalf("failed to create crypto key manager: %v", cryptoErr)
	}

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)

	createdServiceAccount, createErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "sigv4-test-sa",
		Scopes: []string{core.ScopeFileStorageBucketRead, core.ScopeFileStorageObjectRead},
	})
	if createErr != nil {
		t.Fatalf("failed to create service account: %v", createErr)
	}

	serviceAccountUUID, parseErr := uuid.Parse(createdServiceAccount.ID)
	if parseErr != nil {
		t.Fatalf("failed to parse service account id: %v", parseErr)
	}

	accessKeyID := "AKIALAYRTEST12345678"
	secretAccessKey := createdServiceAccount.SecretKey
	encryptedSecretKey, encryptErr := cryptoKeyManager.EncryptField([]byte(secretAccessKey))
	if encryptErr != nil {
		t.Fatalf("failed to encrypt secret key: %v", encryptErr)
	}

	const insertCredentialSQL = `
		INSERT INTO file_storage.s3_credentials (
			service_account_id, access_key_id, encrypted_secret_key
		) VALUES ($1, $2, $3);
	`
	if _, insertErr := db.Exec(ctx, insertCredentialSQL, serviceAccountUUID, accessKeyID, encryptedSecretKey); insertErr != nil {
		t.Fatalf("failed to insert s3 credential: %v", insertErr)
	}

	validator := NewValidator(db, cryptoKeyManager)

	// 1. Valid Header Authentication
	validHeaderRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(validHeaderRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	authenticatedServiceAccount, validateErr := validator.Validate(validHeaderRequest)
	if validateErr != nil {
		t.Fatalf("expected successful header validation, got error: %v", validateErr)
	}
	if authenticatedServiceAccount.ID != createdServiceAccount.ID {
		t.Fatalf("expected service account ID %s, got %s", createdServiceAccount.ID, authenticatedServiceAccount.ID)
	}

	// 2. Valid Query Authentication
	validQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(validQueryRequest, accessKeyID, secretAccessKey, time.Now().UTC(), true)
	authenticatedQueryServiceAccount, validateQueryErr := validator.Validate(validQueryRequest)
	if validateQueryErr != nil {
		t.Fatalf("expected successful query validation, got error: %v", validateQueryErr)
	}
	if authenticatedQueryServiceAccount.ID != createdServiceAccount.ID {
		t.Fatalf("expected query service account ID %s, got %s", createdServiceAccount.ID, authenticatedQueryServiceAccount.ID)
	}

	// 3. Valid with Body reader and X-Amz-Content-Sha256
	payload := []byte("Integration test payload content")
	payloadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "http://example.com/my-bucket/file.txt", bytes.NewReader(payload))
	payloadRequest.Header.Set("X-Amz-Content-Sha256", sha256Hex(payload))
	signTestRequest(payloadRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	payloadServiceAccount, validatePayloadErr := validator.Validate(payloadRequest)
	if validatePayloadErr != nil {
		t.Fatalf("expected successful payload request validation, got: %v", validatePayloadErr)
	}
	if payloadServiceAccount.ID != createdServiceAccount.ID {
		t.Fatalf("unexpected payload account ID: %s", payloadServiceAccount.ID)
	}

	// 4. Invalid Access Key ID
	unknownKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(unknownKeyRequest, "NONEXISTENTACCESSKEYID", secretAccessKey, time.Now().UTC(), false)
	if _, unknownKeyErr := validator.Validate(unknownKeyRequest); !errors.Is(unknownKeyErr, ErrInvalidAccessKeyID) {
		t.Fatalf("expected ErrInvalidAccessKeyID, got: %v", unknownKeyErr)
	}

	// 5. Signature Mismatch
	tamperedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(tamperedRequest, accessKeyID, "wrong-secret-key-000000000000", time.Now().UTC(), false)
	if _, mismatchErr := validator.Validate(tamperedRequest); !errors.Is(mismatchErr, ErrSignatureDoesNotMatch) {
		t.Fatalf("expected ErrSignatureDoesNotMatch, got: %v", mismatchErr)
	}

	// 6. Nil CryptoKeyManager
	nilCryptoValidator := NewValidator(db, nil)
	if _, nilCryptoErr := nilCryptoValidator.Validate(validHeaderRequest); nilCryptoErr == nil || !strings.Contains(nilCryptoErr.Error(), "crypto key manager unavailable") {
		t.Fatalf("expected crypto key manager unavailable error, got: %v", nilCryptoErr)
	}

	// 7. Decrypt Error (corrupted encrypted secret key in database)
	corruptedAccessKeyID := "AKIALAYRCORRUPTED001"
	_, _ = db.Exec(ctx, insertCredentialSQL, serviceAccountUUID, corruptedAccessKeyID, "corrupted:cipher:payload")
	corruptedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(corruptedRequest, corruptedAccessKeyID, secretAccessKey, time.Now().UTC(), false)
	if _, decryptErr := validator.Validate(corruptedRequest); decryptErr == nil || !strings.Contains(decryptErr.Error(), "failed to decrypt") {
		t.Fatalf("expected failed to decrypt error, got: %v", decryptErr)
	}

	// 8. Service Account Disabled
	const disableSQL = `UPDATE core.service_accounts SET is_enabled = false WHERE id = $1;`
	if _, disableErr := db.Exec(ctx, disableSQL, serviceAccountUUID); disableErr != nil {
		t.Fatalf("failed to disable service account: %v", disableErr)
	}
	disabledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(disabledRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	if _, disabledErr := validator.Validate(disabledRequest); !errors.Is(disabledErr, core.ErrServiceAccountDisabled) {
		t.Fatalf("expected ErrServiceAccountDisabled, got: %v", disabledErr)
	}

	// 9. Service Account Expired
	const expireSQL = `UPDATE core.service_accounts SET is_enabled = true, expires_at = now() - interval '1 hour' WHERE id = $1;`
	if _, expireErr := db.Exec(ctx, expireSQL, serviceAccountUUID); expireErr != nil {
		t.Fatalf("failed to expire service account: %v", expireErr)
	}
	expiredRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(expiredRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	if _, expiredErr := validator.Validate(expiredRequest); !errors.Is(expiredErr, core.ErrServiceAccountExpired) {
		t.Fatalf("expected ErrServiceAccountExpired, got: %v", expiredErr)
	}

	// 10. Service Account IP Blocked & Allowed
	const ipSQL = `UPDATE core.service_accounts SET expires_at = NULL, allowed_ips = '{"198.51.100.10"}' WHERE id = $1;`
	if _, updateIPErr := db.Exec(ctx, ipSQL, serviceAccountUUID); updateIPErr != nil {
		t.Fatalf("failed to update allowed_ips: %v", updateIPErr)
	}
	blockedIPRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	blockedIPRequest.Header.Set("X-Forwarded-For", "203.0.113.195")
	signTestRequest(blockedIPRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	if _, ipBlockErr := validator.Validate(blockedIPRequest); !errors.Is(ipBlockErr, core.ErrServiceAccountIPBlocked) {
		t.Fatalf("expected ErrServiceAccountIPBlocked, got: %v", ipBlockErr)
	}

	allowedIPRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	allowedIPRequest.Header.Set("X-Forwarded-For", "198.51.100.10")
	signTestRequest(allowedIPRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	allowedServiceAccount, allowedValidateErr := validator.Validate(allowedIPRequest)
	if allowedValidateErr != nil {
		t.Fatalf("expected allowed IP to succeed, got error: %v", allowedValidateErr)
	}
	if allowedServiceAccount.ID != createdServiceAccount.ID {
		t.Fatalf("unexpected allowed account ID: %s", allowedServiceAccount.ID)
	}

	// 11. Service Account Deleted from DB (cascading credential delete)
	const deleteSQL = `DELETE FROM core.service_accounts WHERE id = $1;`
	if _, deleteAccountErr := db.Exec(ctx, deleteSQL, serviceAccountUUID); deleteAccountErr != nil {
		t.Fatalf("failed to delete service account: %v", deleteAccountErr)
	}
	deletedAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(deletedAccountRequest, accessKeyID, secretAccessKey, time.Now().UTC(), false)
	if _, deletedErr := validator.Validate(deletedAccountRequest); !errors.Is(deletedErr, ErrInvalidAccessKeyID) {
		t.Fatalf("expected ErrInvalidAccessKeyID after cascading delete, got: %v", deletedErr)
	}

	// 12. Orphan S3 Credential (Service Account row not found in DB)
	_, _ = db.Exec(ctx, "ALTER TABLE file_storage.s3_credentials DROP CONSTRAINT IF EXISTS s3_credentials_service_account_id_fkey;")
	orphanAccountUUID := uuid.NewV7()
	orphanAccessKeyID := "AKIALAYRORPHAN12345"
	encryptedOrphanSecret, _ := cryptoKeyManager.EncryptField([]byte("orphan-secret-key-12345678"))
	_, _ = db.Exec(ctx, insertCredentialSQL, orphanAccountUUID, orphanAccessKeyID, encryptedOrphanSecret)
	orphanRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(orphanRequest, orphanAccessKeyID, "orphan-secret-key-12345678", time.Now().UTC(), false)
	if _, orphanErr := validator.Validate(orphanRequest); !errors.Is(orphanErr, core.ErrServiceAccountNotFound) {
		t.Fatalf("expected ErrServiceAccountNotFound for orphan credential, got: %v", orphanErr)
	}

	// 13. Cancelled context during credential query
	canceledCtx, canceledCancel := context.WithCancel(ctx)
	canceledCancel()
	canceledRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	signTestRequest(canceledRequest, orphanAccessKeyID, "orphan-secret-key-12345678", time.Now().UTC(), false)
	if _, cancelQueryErr := validator.Validate(canceledRequest); cancelQueryErr == nil || !strings.Contains(cancelQueryErr.Error(), "failed to query s3 credential") {
		t.Fatalf("expected failed to query s3 credential error on cancelled ctx, got: %v", cancelQueryErr)
	}

	// 14. Empty canonical URI and nil body without X-Amz-Content-Sha256
	secondServiceAccount, _ := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "sigv4-second-sa",
		Scopes: []string{core.ScopeFileStorageBucketRead},
	})
	secondServiceAccountUUID, _ := uuid.Parse(secondServiceAccount.ID)
	secondAccessKeyID := "AKIALAYRSECOND12345"
	secondSecret := secondServiceAccount.SecretKey
	encryptedSecondSecret, _ := cryptoKeyManager.EncryptField([]byte(secondSecret))
	_, _ = db.Exec(ctx, insertCredentialSQL, secondServiceAccountUUID, secondAccessKeyID, encryptedSecondSecret)

	emptyURIRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	emptyURIRequest.URL.Path = ""
	emptyURIRequest.URL.RawPath = ""
	signTestRequest(emptyURIRequest, secondAccessKeyID, secondSecret, time.Now().UTC(), false)
	emptyURIServiceAccount, emptyURIErr := validator.Validate(emptyURIRequest)
	if emptyURIErr != nil {
		t.Fatalf("expected empty URI request to succeed, got: %v", emptyURIErr)
	}
	if emptyURIServiceAccount.ID != secondServiceAccount.ID {
		t.Fatalf("unexpected account ID: %s", emptyURIServiceAccount.ID)
	}

	// 15. Body Read Error
	brokenBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "http://example.com/my-bucket/file.txt", io.NopCloser(badReader{}))
	signTestRequest(brokenBodyRequest, secondAccessKeyID, secondSecret, time.Now().UTC(), false)
	brokenBodyRequest.Body = io.NopCloser(badReader{})
	if _, readBodyErr := validator.Validate(brokenBodyRequest); readBodyErr == nil || !strings.Contains(readBodyErr.Error(), "failed to read body for hashing") {
		t.Fatalf("expected failed to read body error, got: %v", readBodyErr)
	}

	// 16. Explicit nil body without X-Amz-Content-Sha256
	nilBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/my-bucket/file.txt", nil)
	nilBodyRequest.Body = nil
	signTestRequest(nilBodyRequest, secondAccessKeyID, secondSecret, time.Now().UTC(), false)
	nilBodyRequest.Body = nil
	nilBodyServiceAccount, nilBodyErr := validator.Validate(nilBodyRequest)
	if nilBodyErr != nil {
		t.Fatalf("expected nil body request to succeed, got: %v", nilBodyErr)
	}
	if nilBodyServiceAccount.ID != secondServiceAccount.ID {
		t.Fatalf("unexpected account ID: %s", nilBodyServiceAccount.ID)
	}

	// 17. Cancel context during body read to trigger serviceAccountScanErr error (non-ErrNoRows)
	midflightCtx, midflightCancel := context.WithCancel(ctx)
	cancelReader := &cancelingReader{
		midflightCancel: midflightCancel,
		data:            []byte("canceling payload"),
	}
	midflightCancelRequest := httptest.NewRequestWithContext(midflightCtx, http.MethodPut, "http://example.com/my-bucket/file.txt", io.NopCloser(bytes.NewReader([]byte("canceling payload"))))
	signTestRequest(midflightCancelRequest, secondAccessKeyID, secondSecret, time.Now().UTC(), false)
	midflightCancelRequest.Body = io.NopCloser(cancelReader)
	if _, midflightErr := validator.Validate(midflightCancelRequest); midflightErr == nil || !strings.Contains(midflightErr.Error(), "failed querying linked service account") {
		t.Fatalf("expected failed querying linked service account on cancelled ctx, got: %v", midflightErr)
	}
}

type badReader struct{}

func (badReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

type cancelingReader struct {
	midflightCancel context.CancelFunc
	data            []byte
	offset          int
}

func (reader *cancelingReader) Read(buffer []byte) (int, error) {
	if reader.midflightCancel != nil {
		reader.midflightCancel()
	}
	if reader.offset >= len(reader.data) {
		return 0, io.EOF
	}
	bytesToCopy := copy(buffer, reader.data[reader.offset:])
	reader.offset += bytesToCopy
	return bytesToCopy, nil
}
