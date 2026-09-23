package filestorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// DefaultDatabaseChunkSize defines the standard 512KB slice size for the database engine.
const DefaultDatabaseChunkSize = 524288

// DatabaseEngine implements Driver using PostgreSQL BYTEA chunks.
type DatabaseEngine struct {
	kernel         *core.Kernel
	chunkSizeBytes int
}

// DatabaseDriver aliases DatabaseEngine.
type DatabaseDriver = DatabaseEngine

// NewDatabaseEngine initializes a database chunk streaming driver.
func NewDatabaseEngine(kernel *core.Kernel, chunkSizeBytes ...int) *DatabaseEngine {
	size := DefaultDatabaseChunkSize
	if len(chunkSizeBytes) > 0 && chunkSizeBytes[0] > 0 {
		size = chunkSizeBytes[0]
	}
	return &DatabaseEngine{
		kernel:         kernel,
		chunkSizeBytes: size,
	}
}

// NewDatabaseFileStorageEngine initializes an Engine backed by PostgreSQL BYTEA chunks.
func NewDatabaseFileStorageEngine(kernel *core.Kernel, chunkSizeBytes ...int) *Engine {
	return NewEngine(NewDatabaseEngine(kernel, chunkSizeBytes...))
}

// Upload streams binary payload into 512KB chunks within a database transaction.
func (databaseEngine *DatabaseEngine) Upload(
	ctx context.Context,
	bucket Bucket,
	key string,
	reader io.Reader,
	sizeBytes int64,
	contentType string,
) (*Object, error) {
	log.Tracef("uploading object %s to database bucket %s", key, bucket.Name)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	sha256Hash := sha256.New()
	teeReader := io.TeeReader(reader, sha256Hash)

	objectID := uuid.NewV7()
	chunkIndex := 0
	var totalBytes int64

	chunkBuffer := make([]byte, databaseEngine.chunkSizeBytes)

	tx, txErr := databaseEngine.kernel.DB().Begin(ctx)
	if txErr != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", txErr)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const upsertObjectSQL = `
		INSERT INTO file_storage.objects (
			id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, clock_timestamp(), clock_timestamp())
		ON CONFLICT (bucket_id, object_key) DO UPDATE
		SET content_type = EXCLUDED.content_type,
		    size_bytes = EXCLUDED.size_bytes,
		    checksum_sha256 = EXCLUDED.checksum_sha256,
		    last_updated_at = EXCLUDED.last_updated_at
		RETURNING id;
	`

	var targetObjectID uuid.UUID
	scanErr := tx.QueryRow(ctx, upsertObjectSQL, objectID, bucket.ID, key, contentType, 0, "").Scan(&targetObjectID)
	if scanErr != nil {
		return nil, fmt.Errorf("failed to register object record: %w", scanErr)
	}

	// Remove any existing chunks if updating an existing object key
	const deleteOldChunksSQL = `DELETE FROM file_storage.chunks WHERE object_id = $1;`
	_, _ = tx.Exec(ctx, deleteOldChunksSQL, targetObjectID)

	const insertChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, $2, $3);
	`

	for {
		bytesRead, readErr := io.ReadFull(teeReader, chunkBuffer)
		if bytesRead > 0 {
			totalBytes += int64(bytesRead)
			if bucket.MaxFileSizeBytes > 0 && totalBytes > bucket.MaxFileSizeBytes {
				return nil, ErrPayloadTooLarge
			}

			chunkPayload := make([]byte, bytesRead)
			copy(chunkPayload, chunkBuffer[:bytesRead])

			if _, execErr := tx.Exec(ctx, insertChunkSQL, targetObjectID, chunkIndex, chunkPayload); execErr != nil {
				return nil, fmt.Errorf("failed to insert object chunk: %w", execErr)
			}
			chunkIndex++
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
			return nil, fmt.Errorf("failed reading upload stream: %w", readErr)
		}
	}

	checksum := hex.EncodeToString(sha256Hash.Sum(nil))

	const finalizeObjectSQL = `
		UPDATE file_storage.objects
		SET size_bytes = $1,
		    checksum_sha256 = $2,
		    last_updated_at = clock_timestamp()
		WHERE id = $3
		RETURNING id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at;
	`

	var storedObject Object
	_ = tx.QueryRow(ctx, finalizeObjectSQL, totalBytes, checksum, targetObjectID).Scan(
		&storedObject.ID,
		&storedObject.BucketID,
		&storedObject.ObjectKey,
		&storedObject.ContentType,
		&storedObject.SizeBytes,
		&storedObject.ChecksumSHA256,
		&storedObject.Metadata,
		&storedObject.CreatedAt,
		&storedObject.LastUpdatedAt,
	)

	_ = tx.Commit(ctx)

	log.Debugf("object %s stored in database bucket %s (%d bytes)", key, bucket.Name, totalBytes)
	return &storedObject, nil
}

// Head retrieves metadata for an object without streaming its content.
func (databaseEngine *DatabaseEngine) Head(
	ctx context.Context,
	bucket Bucket,
	key string,
) (*Object, error) {
	log.Tracef("heading object %s from database bucket %s", key, bucket.Name)
	const selectObjectSQL = `
		SELECT id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at
		FROM file_storage.objects
		WHERE bucket_id = $1 AND object_key = $2;
	`

	var object Object
	scanErr := databaseEngine.kernel.DB().QueryRow(ctx, selectObjectSQL, bucket.ID, key).Scan(
		&object.ID,
		&object.BucketID,
		&object.ObjectKey,
		&object.ContentType,
		&object.SizeBytes,
		&object.ChecksumSHA256,
		&object.Metadata,
		&object.CreatedAt,
		&object.LastUpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to query object metadata: %w", scanErr)
	}

	return &object, nil
}

// Delete removes an object and its binary chunks from the database.
func (databaseEngine *DatabaseEngine) Delete(
	ctx context.Context,
	bucket Bucket,
	key string,
) error {
	log.Tracef("deleting object %s from database bucket %s", key, bucket.Name)
	const deleteObjectSQL = `
		DELETE FROM file_storage.objects
		WHERE bucket_id = $1 AND object_key = $2;
	`
	deleteResult, execErr := databaseEngine.kernel.DB().Exec(ctx, deleteObjectSQL, bucket.ID, key)
	if execErr != nil {
		return fmt.Errorf("failed to delete object: %w", execErr)
	}
	if deleteResult.RowsAffected() == 0 {
		return ErrObjectNotFound
	}
	return nil
}

// Download returns a readable stream for the object, supporting HTTP Range requests.
func (databaseEngine *DatabaseEngine) Download(
	ctx context.Context,
	bucket Bucket,
	key string,
	contentRange *ContentRange,
) (io.ReadCloser, int64, error) {
	log.Tracef("downloading object %s from database bucket %s", key, bucket.Name)
	object, headErr := databaseEngine.Head(ctx, bucket, key)
	if headErr != nil {
		return nil, 0, headErr
	}

	startByte := int64(0)
	endByte := object.SizeBytes - 1

	if contentRange != nil && contentRange.IsPartial {
		if contentRange.Start < 0 || contentRange.End >= object.SizeBytes || contentRange.Start > contentRange.End {
			return nil, 0, ErrInvalidRange
		}
		startByte = contentRange.Start
		endByte = contentRange.End
	}

	contentLength := endByte - startByte + 1
	if contentLength <= 0 {
		return io.NopCloser(io.LimitReader(nil, 0)), 0, nil
	}

	chunkSize := int64(databaseEngine.chunkSizeBytes)
	startChunkIndex := int(startByte / chunkSize)
	endChunkIndex := int(endByte / chunkSize)

	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer func() {
			_ = pipeWriter.Close()
		}()

		const selectChunksSQL = `
			SELECT chunk_index, chunk_data
			FROM file_storage.chunks
			WHERE object_id = $1 AND chunk_index >= $2 AND chunk_index <= $3
			ORDER BY chunk_index ASC;
		`

		rows, queryErr := databaseEngine.kernel.DB().Query(ctx, selectChunksSQL, object.ID, startChunkIndex, endChunkIndex)
		if queryErr == nil {
			defer rows.Close()

			for rows.Next() {
				var currentChunkIndex int
				var rawChunkData []byte

				_ = rows.Scan(&currentChunkIndex, &rawChunkData)

				chunkSlice := rawChunkData

				// If this is the start chunk, slice off preceding bytes
				if currentChunkIndex == startChunkIndex {
					offsetInChunk := startByte % chunkSize
					if offsetInChunk < int64(len(chunkSlice)) {
						chunkSlice = chunkSlice[offsetInChunk:]
					}
				}

				// If this is the end chunk, slice off trailing bytes
				if currentChunkIndex == endChunkIndex {
					relativeEnd := (endByte % chunkSize) + 1
					if currentChunkIndex == startChunkIndex {
						offsetInChunk := startByte % chunkSize
						relativeEnd -= offsetInChunk
					}
					if relativeEnd >= 0 && relativeEnd < int64(len(chunkSlice)) {
						chunkSlice = chunkSlice[:relativeEnd]
					}
				}

				if len(chunkSlice) > 0 {
					if _, writeErr := pipeWriter.Write(chunkSlice); writeErr != nil {
						return
					}
				}
			}
		}
	}()

	return pipeReader, contentLength, nil
}
