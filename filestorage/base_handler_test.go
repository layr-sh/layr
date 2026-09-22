package filestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageBaseHandlerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := service.BaseHandler()

	t.Run("resolveEngine branches", func(t *testing.T) {
		t.Parallel()

		// Unsupported backend
		_, unsupportedErr := baseHandler.resolveEngine(Bucket{Backend: "gcs"})
		require.Error(t, unsupportedErr)
		require.Contains(t, unsupportedErr.Error(), "unsupported file storage backend")

		// Database backend with valid engine
		dbEngine, validDBErr := baseHandler.resolveEngine(Bucket{Backend: "database"})
		require.NoError(t, validDBErr)
		require.NotNil(t, dbEngine)

		// S3 backend with valid engine
		s3Engine, validS3Err := baseHandler.resolveEngine(Bucket{Backend: "s3"})
		require.NoError(t, validS3Err)
		require.NotNil(t, s3Engine)
	})

	t.Run("presignSecretKey derived from kernel crypto key manager", func(t *testing.T) {
		t.Parallel()
		signingKey := baseHandler.presignSecretKey()
		require.NotEmpty(t, signingKey)
	})

	t.Run("authorizeRESTRequest branches", func(t *testing.T) {
		t.Parallel()
		publicBucket := Bucket{Name: "public-bucket", IsPublic: true}
		privateBucket := Bucket{Name: "private-bucket", IsPublic: false}

		// Public bucket anonymous read -> true
		readRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/file-storage/objects/public-bucket/file.txt", nil)
		require.True(t, baseHandler.authorizeRESTRequest(readRequest, &publicBucket, "file.txt", core.ScopeFileStorageObjectRead))

		// Public bucket anonymous write -> false
		writeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/file-storage/objects/public-bucket/file.txt", nil)
		require.False(t, baseHandler.authorizeRESTRequest(writeRequest, &publicBucket, "file.txt", core.ScopeFileStorageObjectWrite))

		// Private bucket anonymous read -> false
		privateReadRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/file-storage/objects/private-bucket/file.txt", nil)
		require.False(t, baseHandler.authorizeRESTRequest(privateReadRequest, &privateBucket, "file.txt", core.ScopeFileStorageObjectRead))

		// Authenticated user context -> true
		userAuthContext := core.AuthContext{
			UserID: "usr_123",
			JWT: core.JWTClaims{
				Subject: "usr_123",
				Role:    "authenticated",
			},
		}
		userCtx := core.WithAuthContext(context.Background(), userAuthContext)
		userRequest := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/v1/file-storage/objects/private-bucket/file.txt", nil)
		require.True(t, baseHandler.authorizeRESTRequest(userRequest, &privateBucket, "file.txt", core.ScopeFileStorageObjectRead))

		// Service account matching scope
		serviceAccountMatchingAuthContext := core.AuthContext{
			ServiceAccountID: "sa_123",
			JWT: core.JWTClaims{
				Subject: "sa_123",
				Role:    "service_role",
				Scope:   core.ScopeFileStorageObjectRead,
			},
		}
		serviceAccountMatchingCtx := core.WithAuthContext(context.Background(), serviceAccountMatchingAuthContext)
		serviceAccountMatchingRequest := httptest.NewRequestWithContext(serviceAccountMatchingCtx, http.MethodGet, "/v1/file-storage/objects/private-bucket/file.txt", nil)
		require.True(t, baseHandler.authorizeRESTRequest(serviceAccountMatchingRequest, &privateBucket, "file.txt", core.ScopeFileStorageObjectRead))

		// Service account missing scope
		serviceAccountMissingScopeAuthContext := core.AuthContext{
			ServiceAccountID: "sa_123",
			JWT: core.JWTClaims{
				Subject: "sa_123",
				Role:    "service_role",
				Scope:   core.ScopeFileStorageBucketRead,
			},
		}
		serviceAccountMissingScopeCtx := core.WithAuthContext(context.Background(), serviceAccountMissingScopeAuthContext)
		serviceAccountMissingScopeRequest := httptest.NewRequestWithContext(serviceAccountMissingScopeCtx, http.MethodGet, "/v1/file-storage/objects/private-bucket/file.txt", nil)
		require.False(t, baseHandler.authorizeRESTRequest(serviceAccountMissingScopeRequest, &privateBucket, "file.txt", core.ScopeFileStorageObjectRead))
	})

	t.Run("writeS3ErrorResponse writes valid xml", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/file-storage/s3/test", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.writeS3ErrorResponse(responseRecorder, request, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
		require.Equal(t, http.StatusNotFound, responseRecorder.Code)
		require.Contains(t, responseRecorder.Body.String(), "<Code>NoSuchKey</Code>")
		require.Contains(t, responseRecorder.Body.String(), "<Message>The specified key does not exist.</Message>")
	})
}
