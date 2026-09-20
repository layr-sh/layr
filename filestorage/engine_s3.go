package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	cryptoKeyManager *core.CryptoKeyManager
}

// S3Driver aliases S3Engine.
type S3Driver = S3Engine

// NewS3Engine initializes a new S3 proxy driver.
func NewS3Engine(cryptoKeyManager *core.CryptoKeyManager) *S3Engine {
	return &S3Engine{
		cryptoKeyManager: cryptoKeyManager,
	}
}

// NewS3FileStorageEngine initializes an Engine backed by upstream AWS S3.
func NewS3FileStorageEngine(cryptoKeyManager *core.CryptoKeyManager) *Engine {
	return NewEngine(NewS3Engine(cryptoKeyManager))
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
		if s3Engine.cryptoKeyManager == nil {
			return nil, fmt.Errorf("crypto key manager unavailable for decrypting s3 secret")
		}
		decryptedBytes, decryptErr := s3Engine.cryptoKeyManager.DecryptField(rawSecretKey)
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
	s3UpstreamConfiguration, configErr := s3Engine.parseUpstreamConfiguration(bucket)
	if configErr != nil {
		return nil, configErr
	}

	client := s3Engine.getClient(s3UpstreamConfiguration)

	if contentType == "" {
		contentType = "application/octet-stream"
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

	return &Object{
		ID:             uuid.NewV7(),
		BucketID:       bucket.ID,
		ObjectKey:      key,
		ContentType:    contentType,
		SizeBytes:      sizeBytes,
		ChecksumSHA256: etag,
		Metadata:       map[string]any{},
		CreatedAt:      time.Now().UTC(),
		LastUpdatedAt:  time.Now().UTC(),
	}, nil
}

// Head retrieves metadata from upstream S3.
func (s3Engine *S3Engine) Head(
	ctx context.Context,
	bucket Bucket,
	key string,
) (*Object, error) {
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

	return nil
}

// Download forwards stream from upstream S3, honoring ContentRange.
func (s3Engine *S3Engine) Download(
	ctx context.Context,
	bucket Bucket,
	key string,
	contentRange *ContentRange,
) (io.ReadCloser, int64, error) {
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
