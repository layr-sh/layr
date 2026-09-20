package filestorage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"layr.sh/core"
)

// handleDownloadObject handles GET /v1/file-storage/objects/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleDownloadObject(responseWriter http.ResponseWriter, request *http.Request) {
	if baseHandler.configManager != nil && !baseHandler.configManager.Get().Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "download object rejected: file storage is disabled in configuration")
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	if bucketName == "" || objectKey == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket and key parameters are required")
		return
	}

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		if errors.Is(bucketErr, ErrBucketNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Bucket not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "download object rejected: database pool unavailable")
		return
	}

	if !baseHandler.authorizeRESTRequest(request, bucket, objectKey, core.ScopeFileStorageObjectRead) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", engineErr.Error())
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
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Object not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", downloadErr.Error())
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

	_, _ = io.Copy(responseWriter, downloadReadCloser)
}

// handleHeadObject handles HEAD /v1/file-storage/objects/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleHeadObject(responseWriter http.ResponseWriter, request *http.Request) {
	if baseHandler.configManager != nil && !baseHandler.configManager.Get().Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "head object rejected: file storage is disabled in configuration")
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	if bucketName == "" || objectKey == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket and key parameters are required")
		return
	}

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		if errors.Is(bucketErr, ErrBucketNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Bucket not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "head object rejected: database pool unavailable")
		return
	}

	if !baseHandler.authorizeRESTRequest(request, bucket, objectKey, core.ScopeFileStorageObjectRead) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", engineErr.Error())
		return
	}

	object, headErr := fileStorageEngine.Head(ctx, *bucket, objectKey)
	if headErr != nil {
		if errors.Is(headErr, ErrObjectNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Object not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", headErr.Error())
		return
	}

	responseWriter.Header().Set("Content-Type", object.ContentType)
	responseWriter.Header().Set("Content-Length", strconv.FormatInt(object.SizeBytes, 10))
	responseWriter.Header().Set("ETag", fmt.Sprintf("\"%s\"", object.ChecksumSHA256))
	responseWriter.Header().Set("Last-Modified", object.LastUpdatedAt.UTC().Format(http.TimeFormat))
	responseWriter.Header().Set("Accept-Ranges", "bytes")
	responseWriter.WriteHeader(http.StatusOK)
}

// handleUploadObject handles PUT/POST /v1/file-storage/objects/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleUploadObject(responseWriter http.ResponseWriter, request *http.Request) {
	if baseHandler.configManager != nil && !baseHandler.configManager.Get().Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "upload object rejected: file storage is disabled in configuration")
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	if bucketName == "" || objectKey == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket and key parameters are required")
		return
	}

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		if errors.Is(bucketErr, ErrBucketNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Bucket not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "upload object rejected: database pool unavailable")
		return
	}

	if !baseHandler.authorizeRESTRequest(request, bucket, objectKey, core.ScopeFileStorageObjectWrite) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
		return
	}

	contentType := request.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if len(bucket.AllowedMIMETypes) > 0 {
		mimeAllowed := false
		for _, allowedType := range bucket.AllowedMIMETypes {
			if strings.EqualFold(allowedType, contentType) || allowedType == "*/*" {
				mimeAllowed = true
				break
			}
		}
		if !mimeAllowed {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Content type not allowed for this bucket")
			return
		}
	}

	contentLength := request.ContentLength
	if bucket.MaxFileSizeBytes > 0 && contentLength > bucket.MaxFileSizeBytes {
		core.WriteErrorResponse(responseWriter, request, http.StatusRequestEntityTooLarge, "File size exceeds allowed maximum for this bucket")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", engineErr.Error())
		return
	}

	uploadedObject, uploadErr := fileStorageEngine.Upload(ctx, *bucket, objectKey, request.Body, contentLength, contentType)
	if uploadErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", uploadErr.Error())
		return
	}

	baseHandler.writeJSON(responseWriter, http.StatusCreated, uploadedObject)
}

// handleDeleteObject handles DELETE /v1/file-storage/objects/{bucket}/{key...}.
func (baseHandler *BaseHandler) handleDeleteObject(responseWriter http.ResponseWriter, request *http.Request) {
	if baseHandler.configManager != nil && !baseHandler.configManager.Get().Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "delete object rejected: file storage is disabled in configuration")
		return
	}

	bucketName := request.PathValue("bucket")
	objectKey := request.PathValue("key")
	if bucketName == "" || objectKey == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket and key parameters are required")
		return
	}

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, bucketName)
	if bucketErr != nil {
		if errors.Is(bucketErr, ErrBucketNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Bucket not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "delete object rejected: database pool unavailable")
		return
	}

	if !baseHandler.authorizeRESTRequest(request, bucket, objectKey, core.ScopeFileStorageObjectWrite) {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
		return
	}

	fileStorageEngine, engineErr := baseHandler.resolveEngine(*bucket)
	if engineErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", engineErr.Error())
		return
	}

	deleteErr := fileStorageEngine.Delete(ctx, *bucket, objectKey)
	if deleteErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", deleteErr.Error())
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func parseRangeHeader(header string) (*ContentRange, error) {
	if !strings.HasPrefix(header, "bytes=") {
		return nil, fmt.Errorf("invalid range unit")
	}

	spec := strings.TrimPrefix(header, "bytes=")
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range specification")
	}

	start, parseStartErr := strconv.ParseInt(parts[0], 10, 64)
	if parseStartErr != nil {
		return nil, fmt.Errorf("failed to parse range start: %w", parseStartErr)
	}

	end, parseEndErr := strconv.ParseInt(parts[1], 10, 64)
	if parseEndErr != nil {
		return nil, fmt.Errorf("failed to parse range end: %w", parseEndErr)
	}

	if start > end || start < 0 {
		return nil, fmt.Errorf("invalid range values")
	}

	return &ContentRange{
		Start:     start,
		End:       end,
		IsPartial: true,
	}, nil
}
