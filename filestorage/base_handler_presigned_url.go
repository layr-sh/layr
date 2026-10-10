package filestorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

const (
	defaultPresignedExpirationSeconds = 3600
	maxPresignedExpirationSeconds     = 604800
)

// handleCreatePresignedURL handles POST /v1/file-storage/presigned-url.
func (baseHandler *BaseHandler) handleCreatePresignedURL(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling presigned URL request")
	var createPresignedURLInput CreatePresignedURLInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createPresignedURLInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if createPresignedURLInput.Bucket == "" || createPresignedURLInput.Key == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Bucket and key parameters are required")
		return
	}

	operation := strings.ToLower(createPresignedURLInput.Operation)
	switch operation {
	case "get":
		operation = "read"
	case "put":
		operation = "write"
	}

	if operation != "read" && operation != "write" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Operation must be 'read' or 'write'")
		return
	}

	requiredScope := core.ScopeFileStorageObjectRead
	if operation == "write" {
		requiredScope = core.ScopeFileStorageObjectWrite
	}

	// Verify caller identity and scope permissions
	authorized := false
	serviceAccountKey := core.ExtractRequestServiceAccountKey(request)
	if serviceAccountKey != "" {
		clientIP := core.ExtractRequestClientIP(request)
		serviceAccount, authErr := baseHandler.kernel.ServiceAccountManager().Authenticate(request.Context(), serviceAccountKey, clientIP)
		if authErr == nil && core.HasScope(serviceAccount.Scopes, requiredScope) {
			authorized = true
		}
	} else {
		authContext := core.GetAuthContext(request.Context())
		if authContext.IsServiceAccount() {
			authorized = authContext.HasScope(requiredScope)
		} else if authContext.IsAuthenticated() {
			authorized = true
		}
	}

	if !authorized {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
		return
	}

	ctx := request.Context()
	bucket, bucketErr := baseHandler.resolveBucket(ctx, createPresignedURLInput.Bucket)
	if bucketErr != nil {
		if errors.Is(bucketErr, ErrBucketNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Bucket not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", bucketErr.Error())
		return
	}

	expiresInSeconds := createPresignedURLInput.ExpiresInSeconds
	if expiresInSeconds <= 0 {
		expiresInSeconds = defaultPresignedExpirationSeconds
	}
	if expiresInSeconds > maxPresignedExpirationSeconds {
		expiresInSeconds = maxPresignedExpirationSeconds
	}

	expiresAtTime := time.Now().Add(time.Duration(expiresInSeconds) * time.Second).UTC()
	expiresUnix := expiresAtTime.Unix()

	signingKey := baseHandler.presignSecretKey()
	token := baseHandler.computePresignSignature(signingKey, bucket.Name, createPresignedURLInput.Key, operation, expiresUnix)

	presignedURL := fmt.Sprintf("/v1/file-storage/objects/%s/%s?token=%s&expires=%d&op=%s",
		bucket.Name, createPresignedURLInput.Key, token, expiresUnix, operation)

	log.Debugf("presigned url generated for bucket=%s key=%s operation=%s", bucket.Name, createPresignedURLInput.Key, operation)

	baseHandler.kernel.EventBus().Publish(ctx, NewURLPresignedEvent(fmt.Sprintf("%s/%s", bucket.Name, createPresignedURLInput.Key), URLPresignedEventData{
		Bucket:    *bucket,
		ObjectKey: createPresignedURLInput.Key,
		Operation: operation,
		URL:       presignedURL,
		ExpiresAt: expiresAtTime,
	}))

	createPresignedURLResponse := CreatePresignedURLResponse{
		URL:       presignedURL,
		ExpiresAt: expiresAtTime,
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, createPresignedURLResponse)
}
