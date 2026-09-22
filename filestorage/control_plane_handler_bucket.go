package filestorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
)

// handleListBuckets handles GET /v1/_/file-storage/buckets.
func (controlPlaneHandler *ControlPlaneHandler) handleListBuckets(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketRead) {
		return
	}

	ctx := request.Context()
	const querySQL = `
		SELECT id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at
		FROM file_storage.buckets
		ORDER BY name ASC;
	`
	rows, queryErr := controlPlaneHandler.kernel.DB().Query(ctx, querySQL)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	var buckets []Bucket
	for rows.Next() {
		var bucket Bucket
		var rawBackendConfig []byte
		_ = rows.Scan(
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

		bucket.BackendConfig = map[string]any{}
		if len(rawBackendConfig) > 0 {
			_ = json.Unmarshal(rawBackendConfig, &bucket.BackendConfig)
		}

		// Sanitize sensitive upstream secret access key in control plane response
		if secretKey, ok := bucket.BackendConfig["secret_access_key"].(string); ok && secretKey != "" {
			bucket.BackendConfig["secret_access_key"] = "********"
		}

		buckets = append(buckets, bucket)
	}

	listBucketsResponse := ListBucketsResponse{
		Buckets: buckets,
		Count:   len(buckets),
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, listBucketsResponse)
}

// handleCreateBucket handles POST /v1/_/file-storage/buckets.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateBucket(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketWrite) {
		return
	}

	var createBucketInput CreateBucketInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createBucketInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	bucketName := strings.TrimSpace(createBucketInput.Name)
	if bucketName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name is required")
		return
	}

	if !isValidBucketName(bucketName) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name must be 3-63 characters, contain only lowercase letters, numbers, and hyphens, and start/end with an alphanumeric character")
		return
	}

	backend := strings.TrimSpace(createBucketInput.Backend)
	if backend == "" {
		backend = "database"
	}
	if backend != "database" && backend != "s3" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Unsupported backend, must be 'database' or 's3'")
		return
	}

	backendConfig := createBucketInput.BackendConfig
	if backendConfig == nil {
		backendConfig = map[string]any{}
	}

	// Encrypt secret_access_key if backend is S3
	if backend == "s3" {
		if rawSecretKey, ok := backendConfig["secret_access_key"].(string); ok && rawSecretKey != "" {
			if !strings.HasPrefix(rawSecretKey, "enc:v1:aes256gcm:") {
				encryptedSecret, _ := controlPlaneHandler.kernel.CryptoKeyManager().EncryptField([]byte(rawSecretKey))
				backendConfig["secret_access_key"] = encryptedSecret
			}
		}
	}

	rawConfigBytes, _ := json.Marshal(backendConfig)

	maxFileSizeBytes := createBucketInput.MaxFileSizeBytes
	if maxFileSizeBytes <= 0 {
		maxFileSizeBytes = 52428800 // 50MB default
	}

	allowedMIMETypes := createBucketInput.AllowedMIMETypes
	if allowedMIMETypes == nil {
		allowedMIMETypes = []string{}
	}

	bucketID := uuid.NewV7()
	now := time.Now().UTC()

	ctx := request.Context()
	const insertSQL = `
		INSERT INTO file_storage.buckets (
			id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at;
	`

	var createdBucket Bucket
	var returnedConfigBytes []byte
	insertErr := controlPlaneHandler.kernel.DB().QueryRow(
		ctx,
		insertSQL,
		bucketID,
		bucketName,
		createBucketInput.IsPublic,
		backend,
		rawConfigBytes,
		allowedMIMETypes,
		maxFileSizeBytes,
		now,
		now,
	).Scan(
		&createdBucket.ID,
		&createdBucket.Name,
		&createdBucket.IsPublic,
		&createdBucket.Backend,
		&returnedConfigBytes,
		&createdBucket.AllowedMIMETypes,
		&createdBucket.MaxFileSizeBytes,
		&createdBucket.CreatedAt,
		&createdBucket.LastUpdatedAt,
	)
	if insertErr != nil {
		var pgError *pgconn.PgError
		if errors.As(insertErr, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, fmt.Sprintf("Bucket %q already exists", bucketName))
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, insertErr.Error())
		return
	}

	if len(returnedConfigBytes) > 0 {
		_ = json.Unmarshal(returnedConfigBytes, &createdBucket.BackendConfig)
	}
	if secretKey, ok := createdBucket.BackendConfig["secret_access_key"].(string); ok && secretKey != "" {
		createdBucket.BackendConfig["secret_access_key"] = "********"
	}

	core.WriteJSONResponse(responseWriter, http.StatusCreated, createdBucket)
}

// handleGetBucket handles GET /v1/_/file-storage/buckets/{bucket}.
func (controlPlaneHandler *ControlPlaneHandler) handleGetBucket(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketRead) {
		return
	}

	bucketName := request.PathValue("bucket")
	if bucketName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name is required")
		return
	}

	ctx := request.Context()
	const querySQL = `
		SELECT id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at
		FROM file_storage.buckets
		WHERE name = $1;
	`

	var bucket Bucket
	var rawBackendConfig []byte
	queryErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, querySQL, bucketName).Scan(
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
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("Bucket %q not found", bucketName))
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	bucket.BackendConfig = map[string]any{}
	if len(rawBackendConfig) > 0 {
		_ = json.Unmarshal(rawBackendConfig, &bucket.BackendConfig)
	}
	if secretKey, ok := bucket.BackendConfig["secret_access_key"].(string); ok && secretKey != "" {
		bucket.BackendConfig["secret_access_key"] = "********"
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, bucket)
}

// handleUpdateBucket handles PATCH/PUT /v1/_/file-storage/buckets/{bucket}.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateBucket(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketWrite) {
		return
	}

	bucketName := request.PathValue("bucket")
	if bucketName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name is required")
		return
	}

	var updateBucketInput UpdateBucketInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateBucketInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := request.Context()
	const queryCurrentSQL = `
		SELECT id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at
		FROM file_storage.buckets
		WHERE name = $1;
	`
	var bucket Bucket
	var rawBackendConfig []byte
	queryErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, queryCurrentSQL, bucketName).Scan(
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
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("Bucket %q not found", bucketName))
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	if len(rawBackendConfig) > 0 {
		_ = json.Unmarshal(rawBackendConfig, &bucket.BackendConfig)
	}

	if updateBucketInput.IsPublic != nil {
		bucket.IsPublic = *updateBucketInput.IsPublic
	}
	if updateBucketInput.Backend != nil {
		newBackend := *updateBucketInput.Backend
		if newBackend != "database" && newBackend != "s3" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Unsupported backend, must be 'database' or 's3'")
			return
		}
		bucket.Backend = newBackend
	}
	if updateBucketInput.BackendConfig != nil {
		for itemKey, itemValue := range updateBucketInput.BackendConfig {
			if itemKey == "secret_access_key" {
				if secretString, ok := itemValue.(string); ok && secretString != "" && secretString != "********" {
					if !strings.HasPrefix(secretString, "enc:v1:aes256gcm:") {
						encryptedSecret, encryptErr := controlPlaneHandler.kernel.CryptoKeyManager().EncryptField([]byte(secretString))
						if encryptErr == nil {
							itemValue = encryptedSecret
						}
					}
				} else if secretString == "********" {
					continue
				}
			}
			bucket.BackendConfig[itemKey] = itemValue
		}
	}
	if updateBucketInput.AllowedMIMETypes != nil {
		bucket.AllowedMIMETypes = updateBucketInput.AllowedMIMETypes
	}
	if updateBucketInput.MaxFileSizeBytes != nil {
		bucket.MaxFileSizeBytes = *updateBucketInput.MaxFileSizeBytes
	}

	updatedConfigBytes, _ := json.Marshal(bucket.BackendConfig)

	now := time.Now().UTC()
	const updateSQL = `
		UPDATE file_storage.buckets
		SET is_public = $1, backend = $2, backend_config = $3, allowed_mime_types = $4, max_file_size_bytes = $5, last_updated_at = $6
		WHERE id = $7
		RETURNING id, name, is_public, backend, backend_config, allowed_mime_types, max_file_size_bytes, created_at, last_updated_at;
	`
	var updatedBucket Bucket
	var returnedConfigBytes []byte
	_ = controlPlaneHandler.kernel.DB().QueryRow(
		ctx,
		updateSQL,
		bucket.IsPublic,
		bucket.Backend,
		updatedConfigBytes,
		bucket.AllowedMIMETypes,
		bucket.MaxFileSizeBytes,
		now,
		bucket.ID,
	).Scan(
		&updatedBucket.ID,
		&updatedBucket.Name,
		&updatedBucket.IsPublic,
		&updatedBucket.Backend,
		&returnedConfigBytes,
		&updatedBucket.AllowedMIMETypes,
		&updatedBucket.MaxFileSizeBytes,
		&updatedBucket.CreatedAt,
		&updatedBucket.LastUpdatedAt,
	)

	if len(returnedConfigBytes) > 0 {
		_ = json.Unmarshal(returnedConfigBytes, &updatedBucket.BackendConfig)
	}
	if secretKey, ok := updatedBucket.BackendConfig["secret_access_key"].(string); ok && secretKey != "" {
		updatedBucket.BackendConfig["secret_access_key"] = "********"
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, updatedBucket)
}

// handleDeleteBucket handles DELETE /v1/_/file-storage/buckets/{bucket}.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteBucket(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketWrite) {
		return
	}

	bucketName := request.PathValue("bucket")
	if bucketName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name is required")
		return
	}

	ctx := request.Context()
	const deleteSQL = `DELETE FROM file_storage.buckets WHERE name = $1;`
	result, deleteErr := controlPlaneHandler.kernel.DB().Exec(ctx, deleteSQL, bucketName)
	if deleteErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, deleteErr.Error())
		return
	}

	if result.RowsAffected() == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("Bucket %q not found", bucketName))
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleListBucketObjects handles GET /v1/_/file-storage/buckets/{bucket}/objects.
func (controlPlaneHandler *ControlPlaneHandler) handleListBucketObjects(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketRead) {
		return
	}

	bucketName := request.PathValue("bucket")
	if bucketName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket name is required")
		return
	}

	ctx := request.Context()
	const queryBucketSQL = `SELECT id FROM file_storage.buckets WHERE name = $1;`
	var bucketID uuid.UUID
	if err := controlPlaneHandler.kernel.DB().QueryRow(ctx, queryBucketSQL, bucketName).Scan(&bucketID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("Bucket %q not found", bucketName))
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	const queryObjectsSQL = `
		SELECT id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at
		FROM file_storage.objects
		WHERE bucket_id = $1
		ORDER BY object_key ASC;
	`
	objects := []Object{}
	rows, queryErr := controlPlaneHandler.kernel.DB().Query(ctx, queryObjectsSQL, bucketID)
	if queryErr == nil {
		defer rows.Close()
		for rows.Next() {
			var object Object
			var rawMetadata []byte
			_ = rows.Scan(
				&object.ID,
				&object.BucketID,
				&object.ObjectKey,
				&object.ContentType,
				&object.SizeBytes,
				&object.ChecksumSHA256,
				&rawMetadata,
				&object.CreatedAt,
				&object.LastUpdatedAt,
			)
			object.Metadata = map[string]any{}
			if len(rawMetadata) > 0 {
				_ = json.Unmarshal(rawMetadata, &object.Metadata)
			}
			objects = append(objects, object)
		}
	}

	listBucketObjectsResponse := ListBucketObjectsResponse{
		Objects: objects,
		Count:   len(objects),
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, listBucketObjectsResponse)
}

var bucketNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func isValidBucketName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	return bucketNamePattern.MatchString(name)
}
