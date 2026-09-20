package filestorage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
	"layr.sh/filestorage/s3sigv4"
)

// BaseHandler coordinates all public data plane HTTP routes (REST, S3-compatible, and presigned URLs).
type BaseHandler struct {
	db                    *core.DatabasePool
	configManager         *ConfigManager
	cryptoKeyManager      *core.CryptoKeyManager
	serviceAccountManager *core.ServiceAccountManager
	databaseEngine        *Engine
	s3Engine              *Engine
	sigv4Validator        *s3sigv4.Validator
	kvStore               *core.KVStore
	eventBus              *core.EventBus
}

// NewBaseHandler initializes the BaseHandler with engines and validator.
func NewBaseHandler(
	db *core.DatabasePool,
	configManager *ConfigManager,
	cryptoKeyManager *core.CryptoKeyManager,
) *BaseHandler {
	log.Debug("initializing filestorage base handler")
	return &BaseHandler{
		db:                    db,
		configManager:         configManager,
		cryptoKeyManager:      cryptoKeyManager,
		serviceAccountManager: core.NewServiceAccountManager(db),
		databaseEngine:        NewDatabaseFileStorageEngine(db),
		s3Engine:              NewS3FileStorageEngine(cryptoKeyManager),
		sigv4Validator:        s3sigv4.NewValidator(db, cryptoKeyManager),
	}
}

// SetKVStore attaches the KV store reference.
func (baseHandler *BaseHandler) SetKVStore(kvStore *core.KVStore) {
	baseHandler.kvStore = kvStore
}

// SetEventBus attaches the platform event bus.
func (baseHandler *BaseHandler) SetEventBus(eventBus *core.EventBus) {
	baseHandler.eventBus = eventBus
}

// SetServiceAccountManager attaches the service account manager.
func (baseHandler *BaseHandler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	baseHandler.serviceAccountManager = serviceAccountManager
}

// resolveBucket queries bucket metadata by unique bucket name.
func (baseHandler *BaseHandler) resolveBucket(ctx context.Context, bucketName string) (*Bucket, error) {
	if baseHandler.db == nil {
		return nil, fmt.Errorf("database pool unavailable")
	}

	const querySQL = `
		SELECT id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at
		FROM file_storage.buckets
		WHERE name = $1;
	`

	var bucket Bucket
	var rawBackendConfig []byte
	queryErr := baseHandler.db.QueryRow(ctx, querySQL, bucketName).Scan(
		&bucket.ID,
		&bucket.Name,
		&bucket.IsPublic,
		&bucket.Backend,
		&rawBackendConfig,
		&bucket.AllowedMIMETypes,
		&bucket.MaxFileSizeBytes,
		&bucket.CreatedAt,
		&bucket.LastUpdatedAt,
	)
	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil, ErrBucketNotFound
		}
		return nil, fmt.Errorf("failed to query bucket %q: %w", bucketName, queryErr)
	}

	bucket.BackendConfig = map[string]any{}
	if len(rawBackendConfig) > 0 {
		_ = json.Unmarshal(rawBackendConfig, &bucket.BackendConfig)
	}

	return &bucket, nil
}

// resolveEngine returns the appropriate file storage engine for the bucket backend.
func (baseHandler *BaseHandler) resolveEngine(bucket Bucket) (*Engine, error) {
	switch bucket.Backend {
	case "database", "":
		if baseHandler.databaseEngine == nil {
			return nil, fmt.Errorf("database file storage engine unavailable")
		}
		return baseHandler.databaseEngine, nil
	case "s3":
		if baseHandler.s3Engine == nil {
			return nil, fmt.Errorf("s3 file storage engine unavailable")
		}
		return baseHandler.s3Engine, nil
	default:
		return nil, fmt.Errorf("unsupported file storage backend %q", bucket.Backend)
	}
}

// verifyPresignedToken validates an HMAC capability token.
func (baseHandler *BaseHandler) verifyPresignedToken(bucketName string, objectKey string, operation string, token string, expires string) bool {
	if token == "" || expires == "" {
		return false
	}

	expiresUnix, parseErr := strconv.ParseInt(expires, 10, 64)
	if parseErr != nil {
		return false
	}

	if time.Now().Unix() > expiresUnix {
		return false
	}

	signingKey := baseHandler.presignSecretKey()
	expectedSignature := baseHandler.computePresignSignature(signingKey, bucketName, objectKey, operation, expiresUnix)

	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedSignature)) == 1
}

func (baseHandler *BaseHandler) presignSecretKey() []byte {
	if baseHandler.cryptoKeyManager != nil {
		subkey, err := baseHandler.cryptoKeyManager.DeriveSubkey("layr-presign-signing-key")
		if err == nil && len(subkey) > 0 {
			return subkey
		}
	}
	return []byte("layr-default-presign-secret-salt-key-32b")
}

func (baseHandler *BaseHandler) computePresignSignature(signingKey []byte, bucketName string, objectKey string, operation string, expiresUnix int64) string {
	payload := fmt.Sprintf("%s:%s:%s:%d", bucketName, objectKey, operation, expiresUnix)
	hmacHash := hmac.New(sha256.New, signingKey)
	hmacHash.Write([]byte(payload))
	return hex.EncodeToString(hmacHash.Sum(nil))
}

// authorizeRESTRequest evaluates access control for standard REST endpoints.
func (baseHandler *BaseHandler) authorizeRESTRequest(request *http.Request, bucket *Bucket, objectKey string, requiredScope string) bool {
	// 1. Check Presigned Capability Token in query params
	values := request.URL.Query()
	token := values.Get("token")
	expires := values.Get("expires")
	operation := values.Get("op")
	if token != "" && expires != "" {
		expectedOp := "read"
		if requiredScope == core.ScopeFileStorageObjectWrite {
			expectedOp = "write"
		}
		if operation == expectedOp && baseHandler.verifyPresignedToken(bucket.Name, objectKey, operation, token, expires) {
			return true
		}
	}

	// 2. Check Service Account Key (X-Service-Account-Key or Bearer lak_...)
	serviceAccountKey := core.ExtractRequestServiceAccountKey(request)
	if serviceAccountKey != "" && baseHandler.serviceAccountManager != nil {
		clientIP := core.ExtractRequestClientIP(request)
		serviceAccount, authErr := baseHandler.serviceAccountManager.Authenticate(request.Context(), serviceAccountKey, clientIP)
		if authErr == nil && core.HasScope(serviceAccount.Scopes, requiredScope) {
			return true
		}
	}

	// 3. Check AuthContext (e.g. from authenticated JWT or service account middleware)
	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() {
		if authContext.HasScope(requiredScope) {
			return true
		}
	} else if authContext.IsAuthenticated() {
		// Authenticated user has read and write access on standard buckets
		return true
	}

	// 4. Public Bucket Anonymous Access (Read only)
	if bucket.IsPublic && requiredScope == core.ScopeFileStorageObjectRead {
		return true
	}

	return false
}

// writeJSON writes a successful JSON response.
func (baseHandler *BaseHandler) writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

// writeXML writes an XML response.
func (baseHandler *BaseHandler) writeXML(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/xml")
	responseWriter.WriteHeader(statusCode)
	_, _ = responseWriter.Write([]byte(xml.Header))
	_ = xml.NewEncoder(responseWriter).Encode(payload)
}

// writeS3ErrorResponse writes standard S3 XML error envelopes.
func (baseHandler *BaseHandler) writeS3ErrorResponse(responseWriter http.ResponseWriter, request *http.Request, statusCode int, code string, message string) {
	if request.Method == http.MethodHead {
		responseWriter.WriteHeader(statusCode)
		return
	}
	if statusCode >= http.StatusInternalServerError || code == "InternalError" {
		message = "Service temporarily unavailable"
	}
	requestID := request.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "s3-" + uuid.NewV7().String()
	}
	s3ErrorResponse := S3ErrorResponse{
		Code:      code,
		Message:   message,
		Resource:  request.URL.Path,
		RequestID: requestID,
	}
	baseHandler.writeXML(responseWriter, statusCode, s3ErrorResponse)
}
