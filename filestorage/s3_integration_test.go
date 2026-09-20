package filestorage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"layr.sh/core"
)

func TestFilestorageS3Integration(t *testing.T) {
	db, cleanup := setupTestFileStorageDatabase(t)
	defer cleanup()

	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(db)
	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	kvStore := newInMemoryKVStore()
	baseHandler.SetKVStore(kvStore)
	serviceAccountManager := core.NewServiceAccountManager(db)
	baseHandler.SetServiceAccountManager(serviceAccountManager)
	ctx := context.Background()

	// 1. Create Service Account with full S3 permissions
	serviceAccountID, accessKeyID, secretAccessKey := createTestServiceAccountWithS3Credentials(
		t,
		db,
		cryptoKeyManager,
		"s3-standard-client-sa",
		[]string{
			core.ScopeFileStorageBucketRead,
			core.ScopeFileStorageObjectRead,
			core.ScopeFileStorageObjectWrite,
		},
	)

	// 2. Create test bucket in DB
	bucketID := uuid.NewV7()
	const insertBucketSQL = `
		INSERT INTO file_storage.buckets (
			id, name, is_public, backend, allowed_mime_types, max_file_size_bytes
		) VALUES ($1, $2, $3, $4, $5, $6);
	`
	_, insertErr := db.Exec(ctx, insertBucketSQL, bucketID, "test-s3-bucket", false, "database", []string{"text/plain", "application/octet-stream"}, 104857600)
	if insertErr != nil {
		t.Fatalf("failed to insert test bucket: %v", insertErr)
	}

	// 3. Mount S3 endpoints on test server
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("GET /v1/file-storage/s3", baseHandler.handleListS3Buckets)
	serveMux.HandleFunc("GET /v1/file-storage/s3/{bucket}", baseHandler.handleListS3Bucket)
	serveMux.HandleFunc("HEAD /v1/file-storage/s3/{bucket}", baseHandler.handleHeadS3Bucket)
	serveMux.HandleFunc("POST /v1/file-storage/s3/{bucket}", baseHandler.handleDeleteMultipleS3Objects)
	serveMux.HandleFunc("GET /v1/file-storage/s3/{bucket}/{key...}", baseHandler.handleGetS3Object)
	serveMux.HandleFunc("HEAD /v1/file-storage/s3/{bucket}/{key...}", baseHandler.handleHeadS3Object)
	serveMux.HandleFunc("PUT /v1/file-storage/s3/{bucket}/{key...}", baseHandler.handlePutS3Object)
	serveMux.HandleFunc("POST /v1/file-storage/s3/{bucket}/{key...}", baseHandler.handlePostS3Object)
	serveMux.HandleFunc("DELETE /v1/file-storage/s3/{bucket}/{key...}", baseHandler.handleDeleteS3Object)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	// 4. Initialize AWS SDK Go v2 S3 Client
	staticCredentialsProvider := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	awsConfig, loadConfigErr := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(staticCredentialsProvider),
		awsconfig.WithRegion("us-east-1"),
	)
	if loadConfigErr != nil {
		t.Fatalf("failed to load aws config: %v", loadConfigErr)
	}

	s3Client := awss3.NewFromConfig(awsConfig, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(testServer.URL + "/v1/file-storage/s3")
		options.UsePathStyle = true
	})

	// 5. S3 ListBuckets
	listBucketsOutput, listBucketsErr := s3Client.ListBuckets(ctx, &awss3.ListBucketsInput{})
	if listBucketsErr != nil {
		t.Fatalf("failed to list buckets via standard s3 client: %v", listBucketsErr)
	}
	foundBucket := false
	for _, b := range listBucketsOutput.Buckets {
		if *b.Name == "test-s3-bucket" {
			foundBucket = true
			break
		}
	}
	if !foundBucket {
		t.Fatalf("expected test-s3-bucket in list buckets result")
	}

	// 6. S3 HeadBucket
	_, headBucketErr := s3Client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String("test-s3-bucket"),
	})
	if headBucketErr != nil {
		t.Fatalf("failed to head bucket: %v", headBucketErr)
	}

	// 7. S3 PutObject
	fileData := []byte("Hello, AWS S3 Client compatibility test!")
	putObjectOutput, putErr := s3Client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String("test-s3-bucket"),
		Key:         aws.String("standard/hello.txt"),
		Body:        bytes.NewReader(fileData),
		ContentType: aws.String("text/plain"),
	})
	if putErr != nil {
		t.Fatalf("failed to put object via standard s3 client: %v", putErr)
	}
	if putObjectOutput.ETag == nil || *putObjectOutput.ETag == "" {
		t.Fatalf("expected non-empty ETag on put object")
	}

	// 8. S3 HeadObject
	headObjectOutput, headErr := s3Client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("standard/hello.txt"),
	})
	if headErr != nil {
		t.Fatalf("failed to head object via standard s3 client: %v", headErr)
	}
	if *headObjectOutput.ContentLength != int64(len(fileData)) {
		t.Fatalf("expected content length %d, got %d", len(fileData), *headObjectOutput.ContentLength)
	}

	// 9. S3 GetObject
	getObjectOutput, getErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("standard/hello.txt"),
	})
	if getErr != nil {
		t.Fatalf("failed to get object via standard s3 client: %v", getErr)
	}
	defer func() {
		_ = getObjectOutput.Body.Close()
	}()
	downloadedBytes, _ := io.ReadAll(getObjectOutput.Body)
	if string(downloadedBytes) != string(fileData) {
		t.Fatalf("expected %q, got %q", string(fileData), string(downloadedBytes))
	}

	// 10. S3 Multipart Upload Flow
	createMultipartUploadOutput, initMultipartErr := s3Client.CreateMultipartUpload(ctx, &awss3.CreateMultipartUploadInput{
		Bucket:      aws.String("test-s3-bucket"),
		Key:         aws.String("multipart/large.txt"),
		ContentType: aws.String("text/plain"),
	})
	if initMultipartErr != nil {
		t.Fatalf("failed to create multipart upload: %v", initMultipartErr)
	}
	uploadID := *createMultipartUploadOutput.UploadId

	part1Bytes := []byte("First part of multipart data. ")
	part1UploadPartOutput, part1Err := s3Client.UploadPart(ctx, &awss3.UploadPartInput{
		Bucket:     aws.String("test-s3-bucket"),
		Key:        aws.String("multipart/large.txt"),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(part1Bytes),
	})
	if part1Err != nil {
		t.Fatalf("failed to upload part 1: %v", part1Err)
	}

	part2Bytes := []byte("Second part of multipart data.")
	part2UploadPartOutput, part2Err := s3Client.UploadPart(ctx, &awss3.UploadPartInput{
		Bucket:     aws.String("test-s3-bucket"),
		Key:        aws.String("multipart/large.txt"),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(2),
		Body:       bytes.NewReader(part2Bytes),
	})
	if part2Err != nil {
		t.Fatalf("failed to upload part 2: %v", part2Err)
	}

	_, completeErr := s3Client.CompleteMultipartUpload(ctx, &awss3.CompleteMultipartUploadInput{
		Bucket:   aws.String("test-s3-bucket"),
		Key:      aws.String("multipart/large.txt"),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: []s3types.CompletedPart{
				{
					PartNumber: aws.Int32(1),
					ETag:       part1UploadPartOutput.ETag,
				},
				{
					PartNumber: aws.Int32(2),
					ETag:       part2UploadPartOutput.ETag,
				},
			},
		},
	})
	if completeErr != nil {
		t.Fatalf("failed to complete multipart upload: %v", completeErr)
	}

	// Read back completed multipart object
	multipartGetObjectOutput, multipartGetErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("multipart/large.txt"),
	})
	if multipartGetErr != nil {
		t.Fatalf("failed to get completed multipart object: %v", multipartGetErr)
	}
	defer func() {
		_ = multipartGetObjectOutput.Body.Close()
	}()
	assembledBytes, _ := io.ReadAll(multipartGetObjectOutput.Body)
	expectedAssembled := string(part1Bytes) + string(part2Bytes)
	if string(assembledBytes) != expectedAssembled {
		t.Fatalf("expected assembled bytes %q, got %q", expectedAssembled, string(assembledBytes))
	}

	// 11. S3 DeleteObject
	_, deleteErr := s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("standard/hello.txt"),
	})
	if deleteErr != nil {
		t.Fatalf("failed to delete object: %v", deleteErr)
	}

	// 11.1 S3 ListObjectsV2
	listObjectsV2Output, listObjectsErr := s3Client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
		Bucket:  aws.String("test-s3-bucket"),
		Prefix:  aws.String("multipart/"),
		MaxKeys: aws.Int32(10),
	})
	if listObjectsErr != nil {
		t.Fatalf("failed to list objects v2: %v", listObjectsErr)
	}
	if len(listObjectsV2Output.Contents) != 1 || *listObjectsV2Output.Contents[0].Key != "multipart/large.txt" {
		t.Fatalf("unexpected list objects v2 output: %+v", listObjectsV2Output.Contents)
	}

	// 11.2 S3 Range Request (Partial Content)
	rangeGetObjectOutput, rangeGetErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("multipart/large.txt"),
		Range:  aws.String("bytes=0-4"),
	})
	if rangeGetErr != nil {
		t.Fatalf("failed to get object with range: %v", rangeGetErr)
	}
	defer func() {
		_ = rangeGetObjectOutput.Body.Close()
	}()
	rangeBytes, _ := io.ReadAll(rangeGetObjectOutput.Body)
	if string(rangeBytes) != "First" {
		t.Fatalf("expected 'First', got %q", string(rangeBytes))
	}

	// 11.3 S3 Abort Multipart Upload
	abortCreateMultipartUploadInput := &awss3.CreateMultipartUploadInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("aborted/file.txt"),
	}
	abortedCreateMultipartUploadOutput, startAbortErr := s3Client.CreateMultipartUpload(ctx, abortCreateMultipartUploadInput)
	if startAbortErr != nil {
		t.Fatalf("failed to start multipart upload for abort: %v", startAbortErr)
	}
	_, abortErr := s3Client.AbortMultipartUpload(ctx, &awss3.AbortMultipartUploadInput{
		Bucket:   aws.String("test-s3-bucket"),
		Key:      aws.String("aborted/file.txt"),
		UploadId: abortedCreateMultipartUploadOutput.UploadId,
	})
	if abortErr != nil {
		t.Fatalf("failed to abort multipart upload: %v", abortErr)
	}

	// 11.4 S3 DeleteObjects (Multi-delete via POST ?delete)
	deleteObjectsInput := &awss3.DeleteObjectsInput{
		Bucket: aws.String("test-s3-bucket"),
		Delete: &s3types.Delete{
			Objects: []s3types.ObjectIdentifier{
				{Key: aws.String("multipart/large.txt")},
				{Key: aws.String("non-existent-key-for-delete-error.txt")},
			},
		},
	}
	deleteObjectsOutput, multiDeleteErr := s3Client.DeleteObjects(ctx, deleteObjectsInput)
	if multiDeleteErr != nil {
		t.Fatalf("failed to multi-delete objects: %v", multiDeleteErr)
	}
	if len(deleteObjectsOutput.Deleted) != 1 || *deleteObjectsOutput.Deleted[0].Key != "multipart/large.txt" {
		t.Fatalf("unexpected multi-delete output: %+v", deleteObjectsOutput.Deleted)
	}
	if len(deleteObjectsOutput.Errors) != 1 {
		t.Fatalf("expected 1 delete error for non-existent key, got %d", len(deleteObjectsOutput.Errors))
	}

	// 11.5 S3 Not Found Cases
	_, missingGetErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("non-existent-key.txt"),
	})
	if missingGetErr == nil {
		t.Fatalf("expected GetObject on non-existent key to return error")
	}

	_, missingHeadErr := s3Client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("non-existent-key.txt"),
	})
	if missingHeadErr == nil {
		t.Fatalf("expected HeadObject on non-existent key to return error")
	}

	_, missingHeadBucketErr := s3Client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String("non-existent-bucket"),
	})
	if missingHeadBucketErr == nil {
		t.Fatalf("expected HeadBucket on non-existent bucket to return error")
	}

	_, missingListBucketErr := s3Client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
		Bucket: aws.String("non-existent-bucket"),
	})
	if missingListBucketErr == nil {
		t.Fatalf("expected ListObjectsV2 on non-existent bucket to return error")
	}

	_, missingBucketGetErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("non-existent-bucket"),
		Key:    aws.String("file.txt"),
	})
	if missingBucketGetErr == nil {
		t.Fatalf("expected GetObject on non-existent bucket to return error")
	}

	_, missingBucketHeadErr := s3Client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String("non-existent-bucket"),
		Key:    aws.String("file.txt"),
	})
	if missingBucketHeadErr == nil {
		t.Fatalf("expected HeadObject on non-existent bucket to return error")
	}

	_, missingBucketPutErr := s3Client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String("non-existent-bucket"),
		Key:    aws.String("file.txt"),
		Body:   bytes.NewReader([]byte("data")),
	})
	if missingBucketPutErr == nil {
		t.Fatalf("expected PutObject on non-existent bucket to return error")
	}

	_, missingBucketDeleteErr := s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String("non-existent-bucket"),
		Key:    aws.String("file.txt"),
	})
	if missingBucketDeleteErr == nil {
		t.Fatalf("expected DeleteObject on non-existent bucket to return error")
	}

	missingBucketCreateMultipartUploadInput := &awss3.CreateMultipartUploadInput{
		Bucket: aws.String("non-existent-bucket"),
		Key:    aws.String("file.txt"),
	}
	_, missingBucketMultipartErr := s3Client.CreateMultipartUpload(ctx, missingBucketCreateMultipartUploadInput)
	if missingBucketMultipartErr == nil {
		t.Fatalf("expected CreateMultipartUpload on non-existent bucket to return error")
	}

	// 12. S3 Permissions & Scopes Verification: Read-Only Client
	_, readOnlyAccessKeyID, readOnlySecretAccessKey := createTestServiceAccountWithS3Credentials(
		t,
		db,
		cryptoKeyManager,
		"s3-readonly-sa",
		[]string{
			core.ScopeFileStorageObjectRead,
		},
	)
	readOnlyStaticCredentialsProvider := credentials.NewStaticCredentialsProvider(readOnlyAccessKeyID, readOnlySecretAccessKey, "")
	readOnlyConfig, _ := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(readOnlyStaticCredentialsProvider),
		awsconfig.WithRegion("us-east-1"),
	)
	readOnlyS3Client := awss3.NewFromConfig(readOnlyConfig, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(testServer.URL + "/v1/file-storage/s3")
		options.UsePathStyle = true
	})

	// PutObject with read-only client must fail
	_, readOnlyPutErr := readOnlyS3Client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("should-fail.txt"),
		Body:   bytes.NewReader([]byte("fail")),
	})
	if readOnlyPutErr == nil {
		t.Fatalf("expected PutObject with read-only client to fail")
	}

	// 13. Disable service account in database and verify rejection
	const disableServiceAccountSQL = `UPDATE core.service_accounts SET is_enabled = false WHERE id = $1;`
	_, updateDisabledErr := db.Exec(ctx, disableServiceAccountSQL, serviceAccountID)
	if updateDisabledErr != nil {
		t.Fatalf("failed to disable service account: %v", updateDisabledErr)
	}

	_, disabledClientErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("multipart/large.txt"),
	})
	if disabledClientErr == nil {
		t.Fatalf("expected disabled service account to be rejected")
	}

	// 14. Expired service account rejection
	const expireServiceAccountSQL = `UPDATE core.service_accounts SET is_enabled = true, expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1;`
	_, updateExpiredErr := db.Exec(ctx, expireServiceAccountSQL, serviceAccountID)
	if updateExpiredErr != nil {
		t.Fatalf("failed to expire service account: %v", updateExpiredErr)
	}
	_, expiredClientErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("multipart/large.txt"),
	})
	if expiredClientErr == nil {
		t.Fatalf("expected expired service account to be rejected")
	}

	// 15. IP blocked service account rejection
	const blockIPServiceAccountSQL = `UPDATE core.service_accounts SET expires_at = NULL, allowed_ips = '{"10.0.0.1/32"}' WHERE id = $1;`
	_, updateIPErr := db.Exec(ctx, blockIPServiceAccountSQL, serviceAccountID)
	if updateIPErr != nil {
		t.Fatalf("failed to update service account allowed_ips: %v", updateIPErr)
	}
	_, blockedIPClientErr := s3Client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String("test-s3-bucket"),
		Key:    aws.String("multipart/large.txt"),
	})
	if blockedIPClientErr == nil {
		t.Fatalf("expected IP blocked service account to be rejected")
	}

	// Restore service account permissions for parameter edge case tests
	const restoreServiceAccountSQL = `UPDATE core.service_accounts SET allowed_ips = '{}' WHERE id = $1;`
	_, _ = db.Exec(ctx, restoreServiceAccountSQL, serviceAccountID)

	// 16. Multipart parameter validation errors
	// UploadPart invalid uploadId
	_, badUploadIDPartErr := s3Client.UploadPart(ctx, &awss3.UploadPartInput{
		Bucket:     aws.String("test-s3-bucket"),
		Key:        aws.String("multipart/large.txt"),
		UploadId:   aws.String("not-a-uuid"),
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader([]byte("data")),
	})
	if badUploadIDPartErr == nil {
		t.Fatal("expected error on UploadPart with invalid uploadId")
	}

	// UploadPart invalid partNumber (partNumber = 0)
	_, badPartNumberErr := s3Client.UploadPart(ctx, &awss3.UploadPartInput{
		Bucket:     aws.String("test-s3-bucket"),
		Key:        aws.String("multipart/large.txt"),
		UploadId:   aws.String(uuid.NewV7().String()),
		PartNumber: aws.Int32(0),
		Body:       bytes.NewReader([]byte("data")),
	})
	if badPartNumberErr == nil {
		t.Fatal("expected error on UploadPart with invalid partNumber")
	}

	// CompleteMultipartUpload invalid uploadId
	_, badUploadIDCompleteErr := s3Client.CompleteMultipartUpload(ctx, &awss3.CompleteMultipartUploadInput{
		Bucket:   aws.String("test-s3-bucket"),
		Key:      aws.String("multipart/large.txt"),
		UploadId: aws.String("not-a-uuid"),
	})
	if badUploadIDCompleteErr == nil {
		t.Fatal("expected error on CompleteMultipartUpload with invalid uploadId")
	}

	// CompleteMultipartUpload on non-existent bucket
	_, missingBucketCompleteErr := s3Client.CompleteMultipartUpload(ctx, &awss3.CompleteMultipartUploadInput{
		Bucket:   aws.String("non-existent-bucket"),
		Key:      aws.String("multipart/large.txt"),
		UploadId: aws.String(uuid.NewV7().String()),
	})
	if missingBucketCompleteErr == nil {
		t.Fatal("expected error on CompleteMultipartUpload with non-existent bucket")
	}

	// AbortMultipartUpload invalid uploadId
	_, badUploadIDAbortErr := s3Client.AbortMultipartUpload(ctx, &awss3.AbortMultipartUploadInput{
		Bucket:   aws.String("test-s3-bucket"),
		Key:      aws.String("multipart/large.txt"),
		UploadId: aws.String("not-a-uuid"),
	})
	if badUploadIDAbortErr == nil {
		t.Fatal("expected error on AbortMultipartUpload with invalid uploadId")
	}

	// 17. S3 handlers edge cases and error branches
	serviceAccountAuthContext := core.AuthContext{
		ServiceAccountID: serviceAccountID.String(),
		JWT: core.JWTClaims{
			Subject: serviceAccountID.String(),
			Role:    "service_role",
			Scope:   core.ScopeFileStorageBucketRead + " " + core.ScopeFileStorageObjectRead + " " + core.ScopeFileStorageObjectWrite,
		},
	}
	authedCtx := core.WithAuthContext(ctx, serviceAccountAuthContext)

	// 17.1 processDeleteMultipleS3Objects with malformed XML -> 400
	badXMLDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket?delete", bytes.NewReader([]byte("not-xml")))
	badXMLDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	badXMLDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteMultipleS3Objects(badXMLDeleteResponseRecorder, badXMLDeleteRequest)
	if badXMLDeleteResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed xml delete, got %d", badXMLDeleteResponseRecorder.Code)
	}

	// 17.2 processDeleteMultipleS3Objects with valid XML -> 200
	validXMLDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket?delete", bytes.NewReader([]byte("<Delete><Object><Key>notes/hello.txt</Key></Object></Delete>")))
	validXMLDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	validXMLDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteMultipleS3Objects(validXMLDeleteResponseRecorder, validXMLDeleteRequest)
	if validXMLDeleteResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid xml delete, got %d", validXMLDeleteResponseRecorder.Code)
	}

	// 17.3 processUploadPart with failing reader -> 500
	failingReaderPartRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/test-s3-bucket/file.txt?uploadId="+uuid.NewV7().String()+"&partNumber=1", &errorMockReader{})
	failingReaderPartRequest.SetPathValue("bucket", "test-s3-bucket")
	failingReaderPartRequest.SetPathValue("key", "file.txt")
	failingReaderPartResponseRecorder := httptest.NewRecorder()
	baseHandler.processUploadPart(failingReaderPartResponseRecorder, failingReaderPartRequest)
	if failingReaderPartResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing reader upload part, got %d", failingReaderPartResponseRecorder.Code)
	}

	// 17.4 processUploadPart with non-existent uploadId foreign key violation -> 500
	foreignKeyPartRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/test-s3-bucket/file.txt?uploadId="+uuid.NewV7().String()+"&partNumber=1", bytes.NewReader([]byte("data")))
	foreignKeyPartRequest.SetPathValue("bucket", "test-s3-bucket")
	foreignKeyPartRequest.SetPathValue("key", "file.txt")
	foreignKeyPartResponseRecorder := httptest.NewRecorder()
	baseHandler.processUploadPart(foreignKeyPartResponseRecorder, foreignKeyPartRequest)
	if foreignKeyPartResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on foreign key upload part, got %d", foreignKeyPartResponseRecorder.Code)
	}

	// 17.5 Oversized PUT payload -> 400
	oversizedBucketID := uuid.NewV7()
	_, _ = db.Exec(ctx, insertBucketSQL, oversizedBucketID, "test-oversized-bucket", false, "database", []string{"text/plain"}, 10)
	oversizedPutRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/test-oversized-bucket/large.txt", bytes.NewReader([]byte("this payload exceeds ten bytes")))
	oversizedPutRequest.SetPathValue("bucket", "test-oversized-bucket")
	oversizedPutRequest.SetPathValue("key", "large.txt")
	oversizedPutResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePutS3Object(oversizedPutResponseRecorder, oversizedPutRequest)
	if oversizedPutResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on oversized PUT, got %d", oversizedPutResponseRecorder.Code)
	}

	// 17.6 Nil engine errors -> 500
	savedEngine := baseHandler.databaseEngine
	baseHandler.databaseEngine = nil

	nilEngineGetRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	nilEngineGetRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEngineGetRequest.SetPathValue("key", "file.txt")
	nilEngineGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetS3Object(nilEngineGetResponseRecorder, nilEngineGetRequest)
	if nilEngineGetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine get, got %d", nilEngineGetResponseRecorder.Code)
	}

	nilEnginePutRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/test-s3-bucket/file.txt", bytes.NewReader([]byte("test")))
	nilEnginePutRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEnginePutRequest.SetPathValue("key", "file.txt")
	nilEnginePutResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePutS3Object(nilEnginePutResponseRecorder, nilEnginePutRequest)
	if nilEnginePutResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine put, got %d", nilEnginePutResponseRecorder.Code)
	}

	nilEngineDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	nilEngineDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEngineDeleteRequest.SetPathValue("key", "file.txt")
	nilEngineDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteS3Object(nilEngineDeleteResponseRecorder, nilEngineDeleteRequest)
	if nilEngineDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine delete, got %d", nilEngineDeleteResponseRecorder.Code)
	}

	nilEngineMultiDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket?delete", bytes.NewReader([]byte("<Delete><Object><Key>file.txt</Key></Object></Delete>")))
	nilEngineMultiDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEngineMultiDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteMultipleS3Objects(nilEngineMultiDeleteResponseRecorder, nilEngineMultiDeleteRequest)
	if nilEngineMultiDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine multi delete, got %d", nilEngineMultiDeleteResponseRecorder.Code)
	}

	nilEngineCompleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket/file.txt?uploadId="+uuid.NewV7().String(), nil)
	nilEngineCompleteRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEngineCompleteRequest.SetPathValue("key", "file.txt")
	nilEngineCompleteResponseRecorder := httptest.NewRecorder()
	baseHandler.processCompleteMultipartUpload(nilEngineCompleteResponseRecorder, nilEngineCompleteRequest)
	if nilEngineCompleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine complete multipart, got %d", nilEngineCompleteResponseRecorder.Code)
	}

	nilEngineHeadRequest := httptest.NewRequestWithContext(authedCtx, http.MethodHead, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	nilEngineHeadRequest.SetPathValue("bucket", "test-s3-bucket")
	nilEngineHeadRequest.SetPathValue("key", "file.txt")
	nilEngineHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadS3Object(nilEngineHeadResponseRecorder, nilEngineHeadRequest)
	if nilEngineHeadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil engine head, got %d", nilEngineHeadResponseRecorder.Code)
	}

	// 17.7 Failing driver errors -> 500
	baseHandler.databaseEngine = NewEngine(&mockFailingDriver{})

	failingGetRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	failingGetRequest.SetPathValue("bucket", "test-s3-bucket")
	failingGetRequest.SetPathValue("key", "file.txt")
	failingGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetS3Object(failingGetResponseRecorder, failingGetRequest)
	if failingGetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing get, got %d", failingGetResponseRecorder.Code)
	}

	failingHeadRequest := httptest.NewRequestWithContext(authedCtx, http.MethodHead, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	failingHeadRequest.SetPathValue("bucket", "test-s3-bucket")
	failingHeadRequest.SetPathValue("key", "file.txt")
	failingHeadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleHeadS3Object(failingHeadResponseRecorder, failingHeadRequest)
	if failingHeadResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on failing head, got %d", failingHeadResponseRecorder.Code)
	}

	failingPutRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPut, "/v1/file-storage/s3/test-s3-bucket/file.txt", bytes.NewReader([]byte("test")))
	failingPutRequest.SetPathValue("bucket", "test-s3-bucket")
	failingPutRequest.SetPathValue("key", "file.txt")
	failingPutResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePutS3Object(failingPutResponseRecorder, failingPutRequest)
	if failingPutResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing put, got %d", failingPutResponseRecorder.Code)
	}

	failingDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/file-storage/s3/test-s3-bucket/file.txt", nil)
	failingDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	failingDeleteRequest.SetPathValue("key", "file.txt")
	failingDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteS3Object(failingDeleteResponseRecorder, failingDeleteRequest)
	if failingDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing delete, got %d", failingDeleteResponseRecorder.Code)
	}

	failingMultiDeleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket?delete", bytes.NewReader([]byte("<Delete><Object><Key>file.txt</Key></Object></Delete>")))
	failingMultiDeleteRequest.SetPathValue("bucket", "test-s3-bucket")
	failingMultiDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteMultipleS3Objects(failingMultiDeleteResponseRecorder, failingMultiDeleteRequest)
	if failingMultiDeleteResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on failing multi delete with errors array, got %d", failingMultiDeleteResponseRecorder.Code)
	}

	failingUploadID := uuid.NewV7()
	const insertFailingUploadSQL = `INSERT INTO file_storage.multipart_uploads (id, bucket_id, object_key, content_type) VALUES ($1, $2, $3, $4);`
	_, _ = db.Exec(ctx, insertFailingUploadSQL, failingUploadID, bucketID, "file.txt", "text/plain")
	const insertFailingPartSQL = `INSERT INTO file_storage.multipart_parts (upload_id, part_number, etag, size_bytes, chunk_data) VALUES ($1, $2, $3, $4, $5);`
	_, _ = db.Exec(ctx, insertFailingPartSQL, failingUploadID, 1, "etag1", 4, []byte("part"))

	failingCompleteRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket/file.txt?uploadId="+failingUploadID.String(), nil)
	failingCompleteRequest.SetPathValue("bucket", "test-s3-bucket")
	failingCompleteRequest.SetPathValue("key", "file.txt")
	failingCompleteResponseRecorder := httptest.NewRecorder()
	baseHandler.processCompleteMultipartUpload(failingCompleteResponseRecorder, failingCompleteRequest)
	if failingCompleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on failing complete multipart, got %d", failingCompleteResponseRecorder.Code)
	}

	baseHandler.databaseEngine = savedEngine

	// 17.8 Invalid auth algorithm and invalid key
	invalidAlgorithmRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
	invalidAlgorithmRequest.Header.Set("Authorization", "AWS4-HMAC-SHA512 Credential=test/20260921/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	invalidAlgorithmResponseRecorder := httptest.NewRecorder()
	_, invalidAlgorithmAuthorized := baseHandler.authenticateS3(invalidAlgorithmResponseRecorder, invalidAlgorithmRequest, core.ScopeFileStorageBucketRead)
	if invalidAlgorithmAuthorized || invalidAlgorithmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid algorithm, got %d", invalidAlgorithmResponseRecorder.Code)
	}

	nowTimestamp := time.Now().UTC().Format("20060102T150405Z")
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
	invalidKeyRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=nonexistent/"+nowTimestamp[:8]+"/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
	invalidKeyRequest.Header.Set("X-Amz-Date", nowTimestamp)
	invalidKeyResponseRecorder := httptest.NewRecorder()
	_, invalidKeyAuthorized := baseHandler.authenticateS3(invalidKeyResponseRecorder, invalidKeyRequest, core.ScopeFileStorageBucketRead)
	if invalidKeyAuthorized || invalidKeyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on invalid access key, got %d", invalidKeyResponseRecorder.Code)
	}

	mismatchSigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/file-storage/s3", nil)
	mismatchSigRequest.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKeyID+"/"+nowTimestamp[:8]+"/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	mismatchSigRequest.Header.Set("X-Amz-Date", nowTimestamp)
	mismatchSigResponseRecorder := httptest.NewRecorder()
	_, mismatchSigAuthorized := baseHandler.authenticateS3(mismatchSigResponseRecorder, mismatchSigRequest, core.ScopeFileStorageBucketRead)
	if mismatchSigAuthorized || mismatchSigResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on signature mismatch, got %d", mismatchSigResponseRecorder.Code)
	}

	// 17.8 Operations with canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledAuthedCtx := core.WithAuthContext(canceledCtx, serviceAccountAuthContext)

	cancelListBucketsRequest := httptest.NewRequestWithContext(canceledAuthedCtx, http.MethodGet, "/v1/file-storage/s3", nil)
	cancelListBucketsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListS3Buckets(cancelListBucketsResponseRecorder, cancelListBucketsRequest)
	if cancelListBucketsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on cancel list buckets, got %d", cancelListBucketsResponseRecorder.Code)
	}

	cancelListBucketRequest := httptest.NewRequestWithContext(canceledAuthedCtx, http.MethodGet, "/v1/file-storage/s3/test-s3-bucket", nil)
	cancelListBucketRequest.SetPathValue("bucket", "test-s3-bucket")
	cancelListBucketResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListS3Bucket(cancelListBucketResponseRecorder, cancelListBucketRequest)
	if cancelListBucketResponseRecorder.Code != http.StatusNotFound && cancelListBucketResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 404 or 500 on cancel list bucket, got %d", cancelListBucketResponseRecorder.Code)
	}

	cancelCreateMultipartRequest := httptest.NewRequestWithContext(canceledAuthedCtx, http.MethodPost, "/v1/file-storage/s3/test-s3-bucket/file.txt?uploads", nil)
	cancelCreateMultipartRequest.SetPathValue("bucket", "test-s3-bucket")
	cancelCreateMultipartRequest.SetPathValue("key", "file.txt")
	cancelCreateMultipartResponseRecorder := httptest.NewRecorder()
	baseHandler.processCreateMultipartUpload(cancelCreateMultipartResponseRecorder, cancelCreateMultipartRequest)
	if cancelCreateMultipartResponseRecorder.Code != http.StatusNotFound && cancelCreateMultipartResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 404 or 500 on cancel create multipart, got %d", cancelCreateMultipartResponseRecorder.Code)
	}
}
