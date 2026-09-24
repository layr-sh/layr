package filestorage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"uuid"

	"layr.sh/core"
	"layr.sh/filestorage/s3sigv4"
)

func (baseHandler *BaseHandler) authenticateS3(responseWriter http.ResponseWriter, request *http.Request, requiredScope string) (*core.ServiceAccount, bool) {
	if !baseHandler.configManager.Get().Enabled {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Access denied")
		return nil, false
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() {
		if !authContext.HasScope(requiredScope) {
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Insufficient scope permissions for this operation")
			return nil, false
		}
		return &core.ServiceAccount{
			ID:     authContext.ServiceAccountID,
			Scopes: strings.Fields(authContext.JWT.Scope),
		}, true
	}

	serviceAccount, authErr := baseHandler.sigv4Validator.Validate(request)
	if authErr != nil {
		switch {
		case errors.Is(authErr, s3sigv4.ErrMissingAuthHeader):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusUnauthorized, "AccessDenied", "Missing or invalid authorization header")
		case errors.Is(authErr, s3sigv4.ErrInvalidAlgorithm):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Unsupported signature algorithm")
		case errors.Is(authErr, s3sigv4.ErrInvalidAccessKeyID):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "InvalidAccessKeyId", "The AWS Access Key Id you provided does not exist in our records.")
		case errors.Is(authErr, s3sigv4.ErrSignatureDoesNotMatch):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided.")
		case errors.Is(authErr, s3sigv4.ErrRequestExpired):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "RequestTimeTooSkewed", "The difference between the request time and the server's time is too large.")
		case errors.Is(authErr, core.ErrServiceAccountDisabled):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Service account is disabled")
		case errors.Is(authErr, core.ErrServiceAccountExpired):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Service account has expired")
		case errors.Is(authErr, core.ErrServiceAccountIPBlocked):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Client IP address not allowed")
		case errors.Is(authErr, s3sigv4.ErrInvalidTimestamp):
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "AWS authentication requires a valid Date or x-amz-date header")
		default:
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", "Service temporarily unavailable")
		}
		return nil, false
	}

	if !core.HasScope(serviceAccount.Scopes, requiredScope) {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusForbidden, "AccessDenied", "Insufficient scope permissions for this operation")
		return nil, false
	}

	return serviceAccount, true
}

// handleListS3Buckets handles GET /v1/file-storage/s3.
func (baseHandler *BaseHandler) handleListS3Buckets(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleListS3Buckets invoked")
	serviceAccount, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageBucketRead)
	if !authorized {
		return
	}

	ctx := request.Context()
	const querySQL = `
		SELECT name, created_at
		FROM file_storage.buckets
		ORDER BY name ASC;
	`
	rows, queryErr := baseHandler.kernel.DB().Query(ctx, querySQL)
	if queryErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", queryErr.Error())
		return
	}
	defer rows.Close()

	var buckets []S3BucketInfo
	for rows.Next() {
		var s3BucketInfo S3BucketInfo
		if scanErr := rows.Scan(&s3BucketInfo.Name, &s3BucketInfo.CreationDate); scanErr == nil {
			buckets = append(buckets, s3BucketInfo)
		}
	}

	listS3BucketsResponse := ListS3BucketsResponse{
		Owner: S3Owner{
			ID:          serviceAccount.ID,
			DisplayName: serviceAccount.Name,
		},
		Buckets: buckets,
	}

	log.Debugf("retrieved %d s3 bucket(s)", len(buckets))
	baseHandler.writeXML(responseWriter, http.StatusOK, listS3BucketsResponse)
}

// handleHeadS3Bucket handles HEAD /v1/file-storage/s3/{bucket}.
func (baseHandler *BaseHandler) handleHeadS3Bucket(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageBucketRead)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	log.Tracef("handleHeadS3Bucket invoked for bucket %s", bucketName)
	ctx := request.Context()
	_, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	responseWriter.WriteHeader(http.StatusOK)
}

// handleListS3Bucket handles GET /v1/file-storage/s3/{bucket}.
func (baseHandler *BaseHandler) handleListS3Bucket(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageBucketRead)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	log.Tracef("handleListS3Bucket invoked for bucket %s", bucketName)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	values := request.URL.Query()
	prefix := values.Get("prefix")
	maxKeys := 1000
	if maxKeysQuery := values.Get("max-keys"); maxKeysQuery != "" {
		if parsedMaxKeys, parseErr := strconv.Atoi(maxKeysQuery); parseErr == nil && parsedMaxKeys > 0 {
			maxKeys = parsedMaxKeys
		}
	}

	const querySQL = `
		SELECT object_key, last_updated_at, checksum_sha256, size_bytes
		FROM file_storage.objects
		WHERE bucket_id = $1 AND object_key LIKE $2
		ORDER BY object_key ASC
		LIMIT $3;
	`
	var contents []S3ObjectContent
	rows, queryErr := baseHandler.kernel.DB().Query(ctx, querySQL, bucket.ID, prefix+"%", maxKeys)
	if queryErr == nil {
		defer rows.Close()

		for rows.Next() {
			var s3ObjectContent S3ObjectContent
			if scanErr := rows.Scan(&s3ObjectContent.Key, &s3ObjectContent.LastModified, &s3ObjectContent.ETag, &s3ObjectContent.Size); scanErr == nil {
				s3ObjectContent.ETag = fmt.Sprintf("\"%s\"", s3ObjectContent.ETag)
				s3ObjectContent.StorageClass = "STANDARD"
				contents = append(contents, s3ObjectContent)
			}
		}
	}

	listS3BucketResponse := ListS3BucketResponse{
		Name:        bucket.Name,
		Prefix:      prefix,
		KeyCount:    len(contents),
		MaxKeys:     maxKeys,
		IsTruncated: false,
		Contents:    contents,
	}

	baseHandler.writeXML(responseWriter, http.StatusOK, listS3BucketResponse)
}

// handleDeleteMultipleS3Objects handles POST /v1/file-storage/s3/{bucket}?delete.
func (baseHandler *BaseHandler) handleDeleteMultipleS3Objects(responseWriter http.ResponseWriter, request *http.Request) {
	if !request.URL.Query().Has("delete") {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Unsupported operation on bucket")
		return
	}

	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	log.Tracef("handleDeleteMultipleS3Objects invoked for bucket %s", bucketName)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", engineErr.Error())
		return
	}

	var deleteMultipleS3ObjectsInput DeleteMultipleS3ObjectsInput
	if decodeErr := xml.NewDecoder(request.Body).Decode(&deleteMultipleS3ObjectsInput); decodeErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "MalformedXML", "The XML provided was not well-formed.")
		return
	}

	var deletedObjects []S3DeletedObjectConfirmation
	var deleteErrors []S3DeleteError

	for _, objectRef := range deleteMultipleS3ObjectsInput.Objects {
		deleteErr := fileStorageEngine.Delete(ctx, *bucket, objectRef.Key)
		if deleteErr != nil {
			deleteErrors = append(deleteErrors, S3DeleteError{
				Key:     objectRef.Key,
				Code:    "InternalError",
				Message: deleteErr.Error(),
			})
		} else {
			deletedObjects = append(deletedObjects, S3DeletedObjectConfirmation(objectRef))
			baseHandler.kernel.EventBus().Publish(ctx, NewObjectDeletedEvent(fmt.Sprintf("%s/%s", bucket.Name, objectRef.Key), ObjectDeletedEventData{
				BucketName: bucket.Name,
				ObjectKey:  objectRef.Key,
			}))
		}
	}

	deleteMultipleS3ObjectsResponse := DeleteMultipleS3ObjectsResponse{
		Deleted: deletedObjects,
		Errors:  deleteErrors,
	}

	baseHandler.writeXML(responseWriter, http.StatusOK, deleteMultipleS3ObjectsResponse)
}

// handleGetS3Object handles GET /v1/file-storage/s3/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleGetS3Object(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectRead)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	log.Tracef("handleGetS3Object invoked for bucket=%s key=%s", bucketName, objectKey)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", engineErr.Error())
		return
	}

	var contentRange *ContentRange
	rangeHeader := request.Header.Get("Range")
	if rangeHeader != "" {
		parsedContentRange, parseErr := parseRangeHeader(rangeHeader)
		if parseErr == nil {
			contentRange = parsedContentRange
		}
	}

	downloadReadCloser, length, downloadErr := fileStorageEngine.Download(ctx, *bucket, objectKey, contentRange)
	if downloadErr != nil {
		if errors.Is(downloadErr, ErrObjectNotFound) {
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
			return
		}
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", downloadErr.Error())
		return
	}
	defer func() {
		_ = downloadReadCloser.Close()
	}()

	responseWriter.Header().Set("Accept-Ranges", "bytes")
	responseWriter.Header().Set("Content-Length", strconv.FormatInt(length, 10))

	if contentRange != nil && contentRange.IsPartial {
		responseWriter.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", contentRange.Start, contentRange.End, contentRange.Total))
		responseWriter.WriteHeader(http.StatusPartialContent)
	} else {
		responseWriter.WriteHeader(http.StatusOK)
	}

	baseHandler.kernel.EventBus().Publish(ctx, NewObjectDownloadedEvent(fmt.Sprintf("%s/%s", bucket.Name, objectKey), ObjectDownloadedEventData{
		BucketName: bucket.Name,
		ObjectKey:  objectKey,
		SizeBytes:  length,
	}))

	_, _ = io.Copy(responseWriter, downloadReadCloser)
}

// handleHeadS3Object handles HEAD /v1/file-storage/s3/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleHeadS3Object(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectRead)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	log.Tracef("handleHeadS3Object invoked for bucket=%s key=%s", bucketName, objectKey)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	object, headErr := fileStorageEngine.Head(ctx, *bucket, objectKey)
	if headErr != nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	responseWriter.Header().Set("Content-Type", object.ContentType)
	responseWriter.Header().Set("Content-Length", strconv.FormatInt(object.SizeBytes, 10))
	responseWriter.Header().Set("ETag", fmt.Sprintf("\"%s\"", object.ChecksumSHA256))
	responseWriter.Header().Set("Last-Modified", object.LastUpdatedAt.UTC().Format(http.TimeFormat))
	responseWriter.Header().Set("Accept-Ranges", "bytes")
	responseWriter.WriteHeader(http.StatusOK)
}

// handlePutS3Object handles PUT /v1/file-storage/s3/{bucket}/{key...}.
func (baseHandler *BaseHandler) handlePutS3Object(responseWriter http.ResponseWriter, request *http.Request) {
	values := request.URL.Query()
	if values.Has("partNumber") && values.Has("uploadId") {
		baseHandler.processUploadPart(responseWriter, request)
		return
	}

	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	log.Tracef("handlePutS3Object invoked for bucket=%s key=%s", bucketName, objectKey)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	contentType := request.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", engineErr.Error())
		return
	}

	uploadedObject, uploadErr := fileStorageEngine.Upload(ctx, *bucket, objectKey, request.Body, request.ContentLength, contentType)
	if uploadErr != nil {
		if errors.Is(uploadErr, ErrPayloadTooLarge) {
			baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed object size.")
			return
		}
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", uploadErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(ctx, NewObjectUploadedEvent(fmt.Sprintf("%s/%s", bucket.Name, uploadedObject.ObjectKey), ObjectUploadedEventData{
		BucketID:       uploadedObject.BucketID,
		BucketName:     bucket.Name,
		ObjectKey:      uploadedObject.ObjectKey,
		ContentType:    uploadedObject.ContentType,
		SizeBytes:      uploadedObject.SizeBytes,
		ChecksumSHA256: uploadedObject.ChecksumSHA256,
	}))

	responseWriter.Header().Set("ETag", fmt.Sprintf("\"%s\"", uploadedObject.ChecksumSHA256))
	responseWriter.WriteHeader(http.StatusOK)
}

// handlePostS3Object handles POST /v1/file-storage/s3/{bucket}/{key...} (multipart initiation or completion).
func (baseHandler *BaseHandler) handlePostS3Object(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handlePostS3Object invoked")
	values := request.URL.Query()
	if values.Has("uploads") {
		baseHandler.processCreateMultipartUpload(responseWriter, request)
		return
	}
	if values.Has("uploadId") {
		baseHandler.processCompleteMultipartUpload(responseWriter, request)
		return
	}

	baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Missing 'uploads' or 'uploadId' parameter")
}

// handleDeleteS3Object handles DELETE /v1/file-storage/s3/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleDeleteS3Object(responseWriter http.ResponseWriter, request *http.Request) {
	values := request.URL.Query()
	if values.Has("uploadId") {
		baseHandler.processAbortMultipartUpload(responseWriter, request)
		return
	}

	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	log.Tracef("handleDeleteS3Object invoked for bucket=%s key=%s", bucketName, objectKey)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", engineErr.Error())
		return
	}

	if deleteErr := fileStorageEngine.Delete(ctx, *bucket, objectKey); deleteErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", deleteErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(ctx, NewObjectDeletedEvent(fmt.Sprintf("%s/%s", bucket.Name, objectKey), ObjectDeletedEventData{
		BucketName: bucket.Name,
		ObjectKey:  objectKey,
	}))

	responseWriter.WriteHeader(http.StatusNoContent)
}

// processCreateMultipartUpload handles POST /v1/file-storage/s3/{bucket}/{key...}?uploads.
func (baseHandler *BaseHandler) processCreateMultipartUpload(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	log.Tracef("processCreateMultipartUpload invoked for bucket=%s key=%s", bucketName, objectKey)
	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	contentType := request.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	uploadID := uuid.NewV7()
	const insertSQL = `
		INSERT INTO file_storage.multipart_uploads (
			id, bucket_id, object_key, content_type
		) VALUES ($1, $2, $3, $4);
	`
	_, _ = baseHandler.kernel.DB().Exec(ctx, insertSQL, uploadID, bucket.ID, objectKey, contentType)

	baseHandler.kernel.EventBus().Publish(ctx, NewMultipartInitiatedEvent(uploadID.String(), MultipartInitiatedEventData{
		UploadID:   uploadID.String(),
		BucketName: bucket.Name,
		ObjectKey:  objectKey,
	}))

	createS3MultipartUploadResponse := CreateS3MultipartUploadResponse{
		Bucket:   bucket.Name,
		Key:      objectKey,
		UploadID: uploadID.String(),
	}

	baseHandler.writeXML(responseWriter, http.StatusOK, createS3MultipartUploadResponse)
}

// processUploadPart handles PUT /v1/file-storage/s3/{bucket}/{key...}?partNumber=X&uploadId=Y.
func (baseHandler *BaseHandler) processUploadPart(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	values := request.URL.Query()
	uploadIDString := values.Get("uploadId")
	uploadID, parseIDErr := uuid.Parse(uploadIDString)
	if parseIDErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Invalid uploadId parameter")
		return
	}

	partNumber, parsePartErr := strconv.Atoi(values.Get("partNumber"))
	if parsePartErr != nil || partNumber < 1 {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Invalid partNumber parameter")
		return
	}

	log.Tracef("processUploadPart invoked for uploadID=%s part=%d", uploadID, partNumber)

	chunkData, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", readErr.Error())
		return
	}

	md5Sum := md5.Sum(chunkData)
	etag := hex.EncodeToString(md5Sum[:])

	ctx := request.Context()
	const insertPartSQL = `
		INSERT INTO file_storage.multipart_parts (
			upload_id, part_number, etag, size_bytes, chunk_data
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (upload_id, part_number)
		DO UPDATE SET etag = EXCLUDED.etag, size_bytes = EXCLUDED.size_bytes, chunk_data = EXCLUDED.chunk_data;
	`
	_, execErr := baseHandler.kernel.DB().Exec(ctx, insertPartSQL, uploadID, partNumber, etag, int64(len(chunkData)), chunkData)
	if execErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", execErr.Error())
		return
	}

	responseWriter.Header().Set("ETag", fmt.Sprintf("\"%s\"", etag))
	responseWriter.WriteHeader(http.StatusOK)
}

// processCompleteMultipartUpload handles POST /v1/file-storage/s3/{bucket}/{key...}?uploadId=Y.
func (baseHandler *BaseHandler) processCompleteMultipartUpload(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	values := request.URL.Query()
	uploadID, parseErr := uuid.Parse(values.Get("uploadId"))
	if parseErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Invalid uploadId parameter")
		return
	}

	log.Tracef("processCompleteMultipartUpload invoked for uploadID=%s bucket=%s key=%s", uploadID, bucketName, objectKey)

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", engineErr.Error())
		return
	}

	const queryPartsSQL = `
		SELECT chunk_data
		FROM file_storage.multipart_parts
		WHERE upload_id = $1
		ORDER BY part_number ASC;
	`
	const queryTotalSizeSQL = `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM file_storage.multipart_parts
		WHERE upload_id = $1;
	`
	var totalSizeBytes int64
	_ = baseHandler.kernel.DB().QueryRow(ctx, queryTotalSizeSQL, uploadID).Scan(&totalSizeBytes)

	contentType := "application/octet-stream"
	const queryUploadSQL = `SELECT content_type FROM file_storage.multipart_uploads WHERE id = $1;`
	_ = baseHandler.kernel.DB().QueryRow(ctx, queryUploadSQL, uploadID).Scan(&contentType)

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer func() {
			_ = pipeWriter.Close()
		}()
		rows, queryErr := baseHandler.kernel.DB().Query(ctx, queryPartsSQL, uploadID)
		if queryErr == nil {
			defer rows.Close()
			for rows.Next() {
				var partChunk []byte
				if scanErr := rows.Scan(&partChunk); scanErr == nil {
					if _, writeErr := pipeWriter.Write(partChunk); writeErr != nil {
						return
					}
				}
			}
		}
	}()

	uploadedObject, uploadErr := fileStorageEngine.Upload(
		ctx,
		*bucket,
		objectKey,
		pipeReader,
		totalSizeBytes,
		contentType,
	)
	if uploadErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusInternalServerError, "InternalError", uploadErr.Error())
		return
	}

	// Clean up multipart records
	_, _ = baseHandler.kernel.DB().Exec(ctx, `DELETE FROM file_storage.multipart_uploads WHERE id = $1;`, uploadID)

	baseHandler.kernel.EventBus().Publish(ctx, NewMultipartCompletedEvent(uploadID.String(), MultipartCompletedEventData{
		UploadID:       uploadID.String(),
		BucketName:     bucket.Name,
		ObjectKey:      objectKey,
		SizeBytes:      totalSizeBytes,
		ChecksumSHA256: uploadedObject.ChecksumSHA256,
	}))

	baseHandler.kernel.EventBus().Publish(ctx, NewObjectUploadedEvent(fmt.Sprintf("%s/%s", bucket.Name, uploadedObject.ObjectKey), ObjectUploadedEventData{
		BucketID:       uploadedObject.BucketID,
		BucketName:     bucket.Name,
		ObjectKey:      uploadedObject.ObjectKey,
		ContentType:    uploadedObject.ContentType,
		SizeBytes:      uploadedObject.SizeBytes,
		ChecksumSHA256: uploadedObject.ChecksumSHA256,
	}))

	completeS3MultipartUploadResponse := CompleteS3MultipartUploadResponse{
		Bucket:   bucket.Name,
		Key:      objectKey,
		ETag:     fmt.Sprintf("\"%s\"", uploadedObject.ChecksumSHA256),
		Location: fmt.Sprintf("/v1/file-storage/s3/%s/%s", bucket.Name, objectKey),
	}

	baseHandler.writeXML(responseWriter, http.StatusOK, completeS3MultipartUploadResponse)
}

// processAbortMultipartUpload handles DELETE /v1/file-storage/s3/{bucket}/{key...}?uploadId=Y.
func (baseHandler *BaseHandler) processAbortMultipartUpload(responseWriter http.ResponseWriter, request *http.Request) {
	_, authorized := baseHandler.authenticateS3(responseWriter, request, core.ScopeFileStorageObjectWrite)
	if !authorized {
		return
	}

	values := request.URL.Query()
	uploadID, parseErr := uuid.Parse(values.Get("uploadId"))
	if parseErr != nil {
		baseHandler.writeS3ErrorResponse(responseWriter, request, http.StatusBadRequest, "InvalidRequest", "Invalid uploadId parameter")
		return
	}

	log.Tracef("processAbortMultipartUpload invoked for uploadID=%s", uploadID)

	ctx := request.Context()
	_, _ = baseHandler.kernel.DB().Exec(ctx, `DELETE FROM file_storage.multipart_uploads WHERE id = $1;`, uploadID)

	baseHandler.kernel.EventBus().Publish(ctx, NewMultipartAbortedEvent(uploadID.String(), MultipartAbortedEventData{
		UploadID:   uploadID.String(),
		BucketName: request.PathValue("bucket"),
		ObjectKey:  request.PathValue("key"),
	}))

	responseWriter.WriteHeader(http.StatusNoContent)
}
