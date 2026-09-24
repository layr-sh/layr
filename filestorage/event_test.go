package filestorage

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
)

func TestFilestorageEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("creates config updated event", func(t *testing.T) {
		t.Parallel()
		configUpdatedEventData := ConfigUpdatedEventData(DefaultConfig())
		configUpdatedEvent := NewConfigUpdatedEvent("filestorage.config", configUpdatedEventData)
		require.Equal(t, "filestorage.config.updated", configUpdatedEvent.Type)
		require.NotNil(t, configUpdatedEvent.ResourceID)
		require.Equal(t, "filestorage.config", *configUpdatedEvent.ResourceID)
	})

	t.Run("creates bucket created event", func(t *testing.T) {
		t.Parallel()
		bucketCreatedEventData := BucketCreatedEventData{
			Name: "test-bucket",
		}
		bucketCreatedEvent := NewBucketCreatedEvent("bucket-1", bucketCreatedEventData)
		require.Equal(t, "filestorage.bucket.created", bucketCreatedEvent.Type)
		require.NotNil(t, bucketCreatedEvent.ResourceID)
		require.Equal(t, "bucket-1", *bucketCreatedEvent.ResourceID)
	})

	t.Run("creates bucket updated event", func(t *testing.T) {
		t.Parallel()
		bucketUpdatedEventData := BucketUpdatedEventData{
			Name: "test-bucket",
		}
		bucketUpdatedEvent := NewBucketUpdatedEvent("bucket-2", bucketUpdatedEventData)
		require.Equal(t, "filestorage.bucket.updated", bucketUpdatedEvent.Type)
		require.NotNil(t, bucketUpdatedEvent.ResourceID)
		require.Equal(t, "bucket-2", *bucketUpdatedEvent.ResourceID)
	})

	t.Run("creates bucket deleted event", func(t *testing.T) {
		t.Parallel()
		bucketDeletedEventData := BucketDeletedEventData{
			BucketName: "test-bucket",
		}
		bucketDeletedEvent := NewBucketDeletedEvent("bucket-3", bucketDeletedEventData)
		require.Equal(t, "filestorage.bucket.deleted", bucketDeletedEvent.Type)
		require.NotNil(t, bucketDeletedEvent.ResourceID)
		require.Equal(t, "bucket-3", *bucketDeletedEvent.ResourceID)
	})

	t.Run("creates object uploaded event", func(t *testing.T) {
		t.Parallel()
		bucketID := uuid.NewV7()
		objectUploadedEventData := ObjectUploadedEventData{
			BucketID:       bucketID,
			BucketName:     "photos",
			ObjectKey:      "sample.png",
			ContentType:    "image/png",
			SizeBytes:      1024,
			ChecksumSHA256: "abc123sha",
		}
		objectUploadedEvent := NewObjectUploadedEvent("photos/sample.png", objectUploadedEventData)
		require.Equal(t, "filestorage.object.uploaded", objectUploadedEvent.Type)
		require.NotNil(t, objectUploadedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectUploadedEvent.ResourceID)
	})

	t.Run("creates object downloaded event", func(t *testing.T) {
		t.Parallel()
		objectDownloadedEventData := ObjectDownloadedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			SizeBytes:  1024,
		}
		objectDownloadedEvent := NewObjectDownloadedEvent("photos/sample.png", objectDownloadedEventData)
		require.Equal(t, "filestorage.object.downloaded", objectDownloadedEvent.Type)
		require.NotNil(t, objectDownloadedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectDownloadedEvent.ResourceID)
	})

	t.Run("creates object deleted event", func(t *testing.T) {
		t.Parallel()
		objectDeletedEventData := ObjectDeletedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		objectDeletedEvent := NewObjectDeletedEvent("photos/sample.png", objectDeletedEventData)
		require.Equal(t, "filestorage.object.deleted", objectDeletedEvent.Type)
		require.NotNil(t, objectDeletedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectDeletedEvent.ResourceID)
	})

	t.Run("creates object upload failed event", func(t *testing.T) {
		t.Parallel()
		objectUploadFailedEventData := ObjectUploadFailedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			Reason:     "payload too large",
			StatusCode: 413,
		}
		objectUploadFailedEvent := NewObjectUploadFailedEvent("photos/sample.png", objectUploadFailedEventData)
		require.Equal(t, "filestorage.object.upload_failed", objectUploadFailedEvent.Type)
		require.NotNil(t, objectUploadFailedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectUploadFailedEvent.ResourceID)
	})

	t.Run("creates multipart initiated event", func(t *testing.T) {
		t.Parallel()
		multipartInitiatedEventData := MultipartInitiatedEventData{
			UploadID:   "upload-123",
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		multipartInitiatedEvent := NewMultipartInitiatedEvent("upload-123", multipartInitiatedEventData)
		require.Equal(t, "filestorage.multipart.initiated", multipartInitiatedEvent.Type)
		require.NotNil(t, multipartInitiatedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartInitiatedEvent.ResourceID)
	})

	t.Run("creates multipart completed event", func(t *testing.T) {
		t.Parallel()
		multipartCompletedEventData := MultipartCompletedEventData{
			UploadID:       "upload-123",
			BucketName:     "photos",
			ObjectKey:      "sample.png",
			SizeBytes:      2048,
			ChecksumSHA256: "etag123",
		}
		multipartCompletedEvent := NewMultipartCompletedEvent("upload-123", multipartCompletedEventData)
		require.Equal(t, "filestorage.multipart.completed", multipartCompletedEvent.Type)
		require.NotNil(t, multipartCompletedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartCompletedEvent.ResourceID)
	})

	t.Run("creates multipart aborted event", func(t *testing.T) {
		t.Parallel()
		multipartAbortedEventData := MultipartAbortedEventData{
			UploadID:   "upload-123",
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		multipartAbortedEvent := NewMultipartAbortedEvent("upload-123", multipartAbortedEventData)
		require.Equal(t, "filestorage.multipart.aborted", multipartAbortedEvent.Type)
		require.NotNil(t, multipartAbortedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartAbortedEvent.ResourceID)
	})

	t.Run("creates url presigned event", func(t *testing.T) {
		t.Parallel()
		now := time.Now().UTC()
		urlPresignedEventData := URLPresignedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			Operation:  "read",
			URL:        "/v1/file-storage/objects/photos/sample.png?token=xyz",
			ExpiresAt:  now,
		}
		urlPresignedEvent := NewURLPresignedEvent("photos/sample.png", urlPresignedEventData)
		require.Equal(t, "filestorage.url.presigned", urlPresignedEvent.Type)
		require.NotNil(t, urlPresignedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *urlPresignedEvent.ResourceID)
	})
}

func TestFilestorageEventConstructorParameterSignaturesUnit(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse event.go: %v", err)
	}

	testedCount := 0
	for _, decl := range parsedFile.Decls {
		functionDeclaration, ok := decl.(*ast.FuncDecl)
		if !ok || functionDeclaration.Recv != nil {
			continue
		}
		name := functionDeclaration.Name.Name
		if !strings.HasPrefix(name, "New") || !strings.HasSuffix(name, "Event") || name == "NewEvent" {
			continue
		}

		testedCount++
		t.Run(name, func(t *testing.T) {
			params := functionDeclaration.Type.Params.List
			if len(params) == 0 {
				t.Fatalf("constructor %s has no parameters", name)
			}
			firstParamField := params[0]
			if len(firstParamField.Names) == 0 {
				t.Fatalf("constructor %s first parameter has no name", name)
			}
			paramName := firstParamField.Names[0].Name
			if paramName != "resourceID" {
				t.Errorf("constructor %s first parameter expected 'resourceID', got '%s'", name, paramName)
			}
		})
	}

	if testedCount == 0 {
		t.Fatal("no constructors found to test")
	}
}
