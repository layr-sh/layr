package filestorage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"layr.sh/core"
)

func TestFilestorageS3LocalStackIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	localstackContainer, err := localstack.Run(ctx, "localstack/localstack:4")
	if err != nil {
		t.Skipf("skipping localstack integration test (failed to start localstack): %v", err)
		return
	}
	defer func() {
		_ = localstackContainer.Terminate(context.Background())
	}()

	endpoint, err := localstackContainer.PortEndpoint(ctx, "4566/tcp", "http")
	if err != nil {
		t.Fatalf("failed to obtain localstack endpoint: %v", err)
	}

	const testBucketName = "layr-upstream-bucket"
	const region = "us-east-1"
	const accessKey = "test"
	const secretKey = "test"

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("failed to initialize aws config: %v", err)
	}

	directClient := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})

	_, createErr := directClient.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucketName),
	})
	if createErr != nil {
		t.Fatalf("failed to create upstream bucket: %v", createErr)
	}

	kernel := core.NewTestKernel(nil)
	s3Engine := NewS3Engine(kernel)
	testBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "external-s3",
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          endpoint,
			"bucket":            testBucketName,
			"region":            region,
			"access_key_id":     accessKey,
			"secret_access_key": secretKey,
		},
	}

	const testKey = "documents/hello.txt"
	testPayload := []byte("Hello, S3 LocalStack integration engine!")

	// 1. Upload
	uploadedObject, uploadErr := s3Engine.Upload(ctx, testBucket, testKey, bytes.NewReader(testPayload), int64(len(testPayload)), "text/plain")
	if uploadErr != nil {
		t.Fatalf("failed to upload object to upstream s3: %v", uploadErr)
	}
	if uploadedObject.ObjectKey != testKey {
		t.Fatalf("expected key %q, got %q", testKey, uploadedObject.ObjectKey)
	}

	// 1.1 Upload with empty content type -> defaults to application/octet-stream
	_, defaultTypeUploadErr := s3Engine.Upload(ctx, testBucket, "documents/default_mime.txt", bytes.NewReader([]byte("test")), 4, "")
	if defaultTypeUploadErr != nil {
		t.Fatalf("failed to upload with empty content type: %v", defaultTypeUploadErr)
	}
	_ = s3Engine.Delete(ctx, testBucket, "documents/default_mime.txt")

	// 2. Head
	headObject, headErr := s3Engine.Head(ctx, testBucket, testKey)
	if headErr != nil {
		t.Fatalf("failed to head object: %v", headErr)
	}
	if headObject.SizeBytes != int64(len(testPayload)) {
		t.Fatalf("expected size %d, got %d", len(testPayload), headObject.SizeBytes)
	}

	// 3. Download full
	downloadReadCloser, length, downloadErr := s3Engine.Download(ctx, testBucket, testKey, nil)
	if downloadErr != nil {
		t.Fatalf("failed to download object: %v", downloadErr)
	}
	defer func() {
		_ = downloadReadCloser.Close()
	}()

	downloadedBytes, readErr := io.ReadAll(downloadReadCloser)
	if readErr != nil {
		t.Fatalf("failed to read downloaded body: %v", readErr)
	}
	if string(downloadedBytes) != string(testPayload) {
		t.Fatalf("mismatched downloaded content: %q vs %q", string(downloadedBytes), string(testPayload))
	}
	if length != int64(len(testPayload)) {
		t.Fatalf("expected length %d, got %d", len(testPayload), length)
	}

	// 4. Download partial range: bytes=0-4 ("Hello")
	partialContentRange := &ContentRange{
		Start:     0,
		End:       4,
		Total:     int64(len(testPayload)),
		IsPartial: true,
	}
	rangeReadCloser, rangeLength, rangeErr := s3Engine.Download(ctx, testBucket, testKey, partialContentRange)
	if rangeErr != nil {
		t.Fatalf("failed to download partial range: %v", rangeErr)
	}
	defer func() {
		_ = rangeReadCloser.Close()
	}()

	rangeBytes, readRangeErr := io.ReadAll(rangeReadCloser)
	if readRangeErr != nil {
		t.Fatalf("failed to read range body: %v", readRangeErr)
	}
	if string(rangeBytes) != "Hello" {
		t.Fatalf("expected 'Hello', got %q", string(rangeBytes))
	}
	if rangeLength != 5 {
		t.Fatalf("expected range length 5, got %d", rangeLength)
	}

	// 5. Delete
	if deleteErr := s3Engine.Delete(ctx, testBucket, testKey); deleteErr != nil {
		t.Fatalf("failed to delete object: %v", deleteErr)
	}

	// 6. Verify Head after delete returns ErrObjectNotFound
	_, postDeleteHeadErr := s3Engine.Head(ctx, testBucket, testKey)
	if postDeleteHeadErr == nil || !strings.Contains(postDeleteHeadErr.Error(), "object not found") {
		t.Fatalf("expected ErrObjectNotFound after deletion, got: %v", postDeleteHeadErr)
	}

	// 7. Verify Download after delete returns ErrObjectNotFound
	_, _, postDeleteDownloadErr := s3Engine.Download(ctx, testBucket, testKey, nil)
	if postDeleteDownloadErr == nil || !strings.Contains(postDeleteDownloadErr.Error(), "object not found") {
		t.Fatalf("expected ErrObjectNotFound on download after deletion, got: %v", postDeleteDownloadErr)
	}

	// 8. Canceled context on S3 operations (fails getClient)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, cancelUploadErr := s3Engine.Upload(canceledCtx, testBucket, testKey, bytes.NewReader(testPayload), int64(len(testPayload)), "text/plain")
	if cancelUploadErr == nil {
		t.Fatal("expected error on S3 Upload with canceled context")
	}

	_, cancelHeadErr := s3Engine.Head(canceledCtx, testBucket, testKey)
	if cancelHeadErr == nil {
		t.Fatal("expected error on S3 Head with canceled context")
	}

	cancelDeleteErr := s3Engine.Delete(canceledCtx, testBucket, testKey)
	if cancelDeleteErr == nil {
		t.Fatal("expected error on S3 Delete with canceled context")
	}

	_, _, cancelDownloadErr := s3Engine.Download(canceledCtx, testBucket, testKey, nil)
	if cancelDownloadErr == nil {
		t.Fatal("expected error on S3 Download with canceled context")
	}

	// 9. Unreachable S3 endpoint (network failures on upstream SDK calls)
	unreachableBucket := Bucket{
		ID:      testBucket.ID,
		Name:    testBucket.Name,
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          "http://127.0.0.1:1",
			"bucket":            testBucketName,
			"region":            "us-east-1",
			"access_key_id":     "mock-key",
			"secret_access_key": "mock-secret",
		},
	}

	_, unreachableUploadErr := s3Engine.Upload(ctx, unreachableBucket, testKey, bytes.NewReader(testPayload), int64(len(testPayload)), "text/plain")
	if unreachableUploadErr == nil {
		t.Fatal("expected error on S3 Upload with unreachable endpoint")
	}

	_, unreachableHeadErr := s3Engine.Head(ctx, unreachableBucket, testKey)
	if unreachableHeadErr == nil {
		t.Fatal("expected error on S3 Head with unreachable endpoint")
	}

	unreachableDeleteErr := s3Engine.Delete(ctx, unreachableBucket, testKey)
	if unreachableDeleteErr == nil {
		t.Fatal("expected error on S3 Delete with unreachable endpoint")
	}

	_, _, unreachableDownloadErr := s3Engine.Download(ctx, unreachableBucket, testKey, nil)
	if unreachableDownloadErr == nil {
		t.Fatal("expected error on S3 Download with unreachable endpoint")
	}
}
