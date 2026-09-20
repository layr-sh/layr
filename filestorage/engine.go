package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Standard file storage error definitions.
var (
	ErrObjectNotFound       = errors.New("object not found")
	ErrBucketNotFound       = errors.New("bucket not found")
	ErrInvalidRange         = errors.New("invalid content range")
	ErrPayloadTooLarge      = errors.New("payload exceeds maximum allowed file size")
	ErrUnsupportedBackend   = errors.New("unsupported file storage backend")
	ErrEngineNotInitialized = errors.New("file storage engine is not initialized")
)

// ContentRange specifies a byte range for partial downloads.
type ContentRange struct {
	Start     int64
	End       int64
	Total     int64
	IsPartial bool
}

// Driver defines the pluggable file storage driver interface.
type Driver interface {
	// Upload streams binary data from reader into the target bucket and object key.
	Upload(
		ctx context.Context,
		bucket Bucket,
		key string,
		reader io.Reader,
		sizeBytes int64,
		contentType string,
	) (*Object, error)

	// Download returns a readable stream for the specified object, supporting partial content ranges.
	Download(
		ctx context.Context,
		bucket Bucket,
		key string,
		contentRange *ContentRange,
	) (io.ReadCloser, int64, error)

	// Delete removes an object and its associated file storage assets.
	Delete(ctx context.Context, bucket Bucket, key string) error

	// Head retrieves metadata for an object without reading its payload.
	Head(ctx context.Context, bucket Bucket, key string) (*Object, error)
}

// Engine represents the file storage manager struct.
type Engine struct {
	driver Driver
}

// NewEngine initializes an Engine struct wrapping a Driver.
func NewEngine(driver Driver) *Engine {
	if driver == nil {
		return nil
	}
	return &Engine{driver: driver}
}

// Driver returns the underlying Driver.
func (engine *Engine) Driver() Driver {
	if engine == nil {
		return nil
	}
	return engine.driver
}

// Upload delegates to the underlying file storage driver.
func (engine *Engine) Upload(
	ctx context.Context,
	bucket Bucket,
	key string,
	reader io.Reader,
	sizeBytes int64,
	contentType string,
) (*Object, error) {
	if engine == nil || engine.driver == nil {
		return nil, ErrEngineNotInitialized
	}
	uploadedObject, uploadErr := engine.driver.Upload(ctx, bucket, key, reader, sizeBytes, contentType)
	if uploadErr != nil {
		return nil, fmt.Errorf("%w", uploadErr)
	}
	return uploadedObject, nil
}

// Download delegates to the underlying file storage driver.
func (engine *Engine) Download(
	ctx context.Context,
	bucket Bucket,
	key string,
	contentRange *ContentRange,
) (io.ReadCloser, int64, error) {
	if engine == nil || engine.driver == nil {
		return nil, 0, ErrEngineNotInitialized
	}
	downloadReadCloser, length, downloadErr := engine.driver.Download(ctx, bucket, key, contentRange)
	if downloadErr != nil {
		return nil, 0, fmt.Errorf("%w", downloadErr)
	}
	return downloadReadCloser, length, nil
}

// Delete delegates to the underlying file storage driver.
func (engine *Engine) Delete(ctx context.Context, bucket Bucket, key string) error {
	if engine == nil || engine.driver == nil {
		return ErrEngineNotInitialized
	}
	if deleteErr := engine.driver.Delete(ctx, bucket, key); deleteErr != nil {
		return fmt.Errorf("%w", deleteErr)
	}
	return nil
}

// Head delegates to the underlying file storage driver.
func (engine *Engine) Head(ctx context.Context, bucket Bucket, key string) (*Object, error) {
	if engine == nil || engine.driver == nil {
		return nil, ErrEngineNotInitialized
	}
	headObject, headErr := engine.driver.Head(ctx, bucket, key)
	if headErr != nil {
		return nil, fmt.Errorf("%w", headErr)
	}
	return headObject, nil
}
