package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"layr.sh/core"
)

// S3Engine forwards file storage operations to upstream AWS S3 or compatible endpoints.
type S3Engine struct {
	kernel *core.Kernel
}

// NewS3Engine initializes a new S3 proxy driver.
func NewS3Engine(kernel *core.Kernel) *S3Engine {
	return &S3Engine{
		kernel: kernel,
	}
}

// NewS3FileStorageEngine initializes an Engine backed by upstream AWS S3.
func NewS3FileStorageEngine(kernel *core.Kernel) *Engine {
	return NewEngine(NewS3Engine(kernel))
}

// S3UpstreamConfiguration stores upstream S3 connection parameters.
type S3UpstreamConfiguration struct {
	Endpoint        string
	BucketName      string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

func (s3Engine *S3Engine) parseUpstreamConfiguration(bucket Bucket) (*S3UpstreamConfiguration, error) {
	configMap := bucket.BackendConfig
	if configMap == nil {
		return nil, fmt.Errorf("missing backend_config for s3 bucket %s", bucket.Name)
	}

	targetBucket, _ := configMap["bucket"].(string)
	if targetBucket == "" {
		targetBucket = bucket.Name
	}

	endpoint, _ := configMap["endpoint"].(string)
	region, _ := configMap["region"].(string)
	if region == "" {
		region = "us-east-1"
	}

	accessKeyID, _ := configMap["access_key_id"].(string)
	rawSecretKey, _ := configMap["secret_access_key"].(string)

	secretAccessKey := rawSecretKey
	if strings.HasPrefix(rawSecretKey, "enc:v1:aes256gcm:") {
		decryptedBytes, decryptErr := s3Engine.kernel.CryptoKeyManager().DecryptField(rawSecretKey)
		if decryptErr != nil {
			return nil, fmt.Errorf("failed to decrypt s3 secret_access_key: %w", decryptErr)
		}
		secretAccessKey = string(decryptedBytes)
	}

	return &S3UpstreamConfiguration{
		Endpoint:        endpoint,
		BucketName:      targetBucket,
		Region:          region,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}, nil
}

func (s3Engine *S3Engine) getClient(s3UpstreamConfiguration *S3UpstreamConfiguration) *s3.Client {
	awsConfig := aws.Config{
		Region: s3UpstreamConfiguration.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			s3UpstreamConfiguration.AccessKeyID,
			s3UpstreamConfiguration.SecretAccessKey,
			"",
		),
	}

	return s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if s3UpstreamConfiguration.Endpoint != "" {
			options.BaseEndpoint = aws.String(s3UpstreamConfiguration.Endpoint)
		}
		options.UsePathStyle = true
	})
}

// Upload forwards streaming payload to upstream S3.
func (s3Engine *S3Engine) Upload(
	ctx context.Context,
	bucket Bucket,
	key string,
	reader io.Reader,
	sizeBytes int64,
	contentType string,
) (*Object, error) {
	log.Tracef("uploading object %s to upstream S3 bucket %s", key, bucket.Name)
	s3UpstreamConfiguration, configErr := s3Engine.parseUpstreamConfiguration(bucket)
	if configErr != nil {
		return nil, configErr
	}

	client := s3Engine.getClient(s3UpstreamConfiguration)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Stage bounded uploads so unknown or inaccurate lengths cannot bypass bucket limits.
	if bucket.MaxFileSizeBytes > 0 || sizeBytes < 0 {
		file, err := os.CreateTemp("", "layr-file-storage-*")
		if err != nil {
			return nil, fmt.Errorf("failed to stage upload: %w", err)
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		sourceReader := reader
		if bucket.MaxFileSizeBytes > 0 {
			sourceReader = io.LimitReader(reader, bucket.MaxFileSizeBytes)
		}
		copied, copyErr := io.Copy(file, sourceReader)
		if copyErr != nil {
			return nil, fmt.Errorf("failed to stage upload: %w", copyErr)
		}
		if bucket.MaxFileSizeBytes > 0 && copied == bucket.MaxFileSizeBytes {
			var extra [1]byte
			if n, err := io.ReadFull(reader, extra[:]); n > 0 {
				return nil, ErrPayloadTooLarge
			} else if err != nil && !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("failed reading upload: %w", err)
			}
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to rewind upload: %w", err)
		}
		reader = io.Reader(file)
		sizeBytes = copied
	}

	putObjectInput := &s3.PutObjectInput{
		Bucket:        aws.String(s3UpstreamConfiguration.BucketName),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
	}

	putObjectOutput, putErr := client.PutObject(ctx, putObjectInput)
	if putErr != nil {
		return nil, fmt.Errorf("upstream s3 put object failed: %w", putErr)
	}

	etag := ""
	if putObjectOutput.ETag != nil {
		etag = strings.Trim(*putObjectOutput.ETag, "\"")
	}

	object := &Object{
		ID: uuid.NewV7(), BucketID: bucket.ID, ObjectKey: key,
		ContentType: contentType, SizeBytes: sizeBytes, ChecksumSHA256: etag,
		Metadata: map[string]any{}, CreatedAt: time.Now().UTC(), LastUpdatedAt: time.Now().UTC(),
	}
	if s3Engine.kernel.DB() != nil {
		const upsertSQL = `
			INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb)
			ON CONFLICT (bucket_id, object_key) DO UPDATE
			SET content_type = EXCLUDED.content_type, size_bytes = EXCLUDED.size_bytes,
			    checksum_sha256 = EXCLUDED.checksum_sha256, last_updated_at = clock_timestamp()
			RETURNING id, created_at, last_updated_at;
		`
		if err := s3Engine.kernel.DB().QueryRow(ctx, upsertSQL, object.ID, bucket.ID, key, contentType, sizeBytes, etag).
			Scan(&object.ID, &object.CreatedAt, &object.LastUpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to register upstream object metadata: %w", err)
		}
	}
	return object, nil
}

// Head retrieves metadata from upstream S3.
func (s3Engine *S3Engine) Head(
	ctx context.Context,
	bucket Bucket,
	key string,
) (*Object, error) {
	log.Tracef("heading object %s from upstream S3 bucket %s", key, bucket.Name)
	s3UpstreamConfiguration, configErr := s3Engine.parseUpstreamConfiguration(bucket)
	if configErr != nil {
		return nil, configErr
	}

	client := s3Engine.getClient(s3UpstreamConfiguration)

	headObjectInput := &s3.HeadObjectInput{
		Bucket: aws.String(s3UpstreamConfiguration.BucketName),
		Key:    aws.String(key),
	}

	headObjectOutput, headErr := client.HeadObject(ctx, headObjectInput)
	if headErr != nil {
		var notFound *s3types.NotFound
		var noSuchKey *s3types.NoSuchKey
		if errors.As(headErr, &notFound) || errors.As(headErr, &noSuchKey) || strings.Contains(headErr.Error(), "404") {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("upstream s3 head object failed: %w", headErr)
	}

	size := int64(0)
	if headObjectOutput.ContentLength != nil {
		size = *headObjectOutput.ContentLength
	}

	contentType := "application/octet-stream"
	if headObjectOutput.ContentType != nil {
		contentType = *headObjectOutput.ContentType
	}

	etag := ""
	if headObjectOutput.ETag != nil {
		etag = strings.Trim(*headObjectOutput.ETag, "\"")
	}

	return &Object{
		ID:             uuid.NewV7(),
		BucketID:       bucket.ID,
		ObjectKey:      key,
		ContentType:    contentType,
		SizeBytes:      size,
		ChecksumSHA256: etag,
		Metadata:       map[string]any{},
		CreatedAt:      time.Now().UTC(),
		LastUpdatedAt:  time.Now().UTC(),
	}, nil
}

// Delete removes the object from upstream S3.
func (s3Engine *S3Engine) Delete(
	ctx context.Context,
	bucket Bucket,
	key string,
) error {
	log.Tracef("deleting object %s from upstream S3 bucket %s", key, bucket.Name)
	s3UpstreamConfiguration, configErr := s3Engine.parseUpstreamConfiguration(bucket)
	if configErr != nil {
		return configErr
	}

	client := s3Engine.getClient(s3UpstreamConfiguration)

	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(s3UpstreamConfiguration.BucketName),
		Key:    aws.String(key),
	}

	if _, deleteErr := client.DeleteObject(ctx, deleteObjectInput); deleteErr != nil {
		return fmt.Errorf("upstream s3 delete object failed: %w", deleteErr)
	}

	if s3Engine.kernel.DB() != nil {
		if _, err := s3Engine.kernel.DB().Exec(ctx, `DELETE FROM file_storage.objects WHERE bucket_id = $1 AND object_key = $2`, bucket.ID, key); err != nil {
			return fmt.Errorf("failed to delete upstream object metadata: %w", err)
		}
	}
	return nil
}

// Download forwards stream from upstream S3, honoring ContentRange.
func (s3Engine *S3Engine) Download(
	ctx context.Context,
	bucket Bucket,
	key string,
	contentRange *ContentRange,
) (io.ReadCloser, int64, error) {
	log.Tracef("downloading object %s from upstream S3 bucket %s", key, bucket.Name)
	s3UpstreamConfiguration, configErr := s3Engine.parseUpstreamConfiguration(bucket)
	if configErr != nil {
		return nil, 0, configErr
	}

	client := s3Engine.getClient(s3UpstreamConfiguration)

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s3UpstreamConfiguration.BucketName),
		Key:    aws.String(key),
	}

	if contentRange != nil && contentRange.IsPartial {
		rangeHeader := fmt.Sprintf("bytes=%d-%d", contentRange.Start, contentRange.End)
		getObjectInput.Range = aws.String(rangeHeader)
	}

	getObjectOutput, getErr := client.GetObject(ctx, getObjectInput)
	if getErr != nil {
		var notFound *s3types.NotFound
		var noSuchKey *s3types.NoSuchKey
		if errors.As(getErr, &notFound) || errors.As(getErr, &noSuchKey) || strings.Contains(getErr.Error(), "404") {
			return nil, 0, ErrObjectNotFound
		}
		return nil, 0, fmt.Errorf("upstream s3 get object failed: %w", getErr)
	}

	length := int64(0)
	if getObjectOutput.ContentLength != nil {
		length = *getObjectOutput.ContentLength
	}

	return getObjectOutput.Body, length, nil
}
