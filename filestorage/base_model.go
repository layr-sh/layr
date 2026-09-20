// Package filestorage defines core models, configurations, and handlers for the file storage service.
package filestorage

import (
	"encoding/xml"
	"time"
	"uuid"
)

// Object represents a stored binary asset entity in file_storage.objects.
type Object struct {
	ID             uuid.UUID      `json:"id"`
	BucketID       uuid.UUID      `json:"bucket_id"`
	ObjectKey      string         `json:"object_key"`
	ContentType    string         `json:"content_type"`
	SizeBytes      int64          `json:"size_bytes"`
	ChecksumSHA256 string         `json:"checksum_sha256"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	LastUpdatedAt  time.Time      `json:"last_updated_at"`
}

// PresignURLInput specifies parameters for generating a capability token and URL.
type PresignURLInput struct {
	Bucket           string `json:"bucket"`
	Key              string `json:"key"`
	Operation        string `json:"operation"` // "GET" or "PUT"
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// PresignURLResponse represents the generated presigned capability URL.
type PresignURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// S3BucketInfo represents bucket metadata within S3 XML listings.
type S3BucketInfo struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

// S3Owner represents owner information in S3 XML responses.
type S3Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// ListS3BucketsResponse represents the S3 ListBuckets XML response envelope.
type ListS3BucketsResponse struct {
	XMLName xml.Name       `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListAllMyBucketsResult"`
	Owner   S3Owner        `xml:"Owner"`
	Buckets []S3BucketInfo `xml:"Buckets>Bucket"`
}

// S3ObjectContent represents single object metadata in S3 ListBucket responses.
type S3ObjectContent struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
}

// ListS3BucketResponse represents the S3 ListObjectsV2 XML response envelope.
type ListS3BucketResponse struct {
	XMLName        xml.Name          `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name           string            `xml:"Name"`
	Prefix         string            `xml:"Prefix"`
	KeyCount       int               `xml:"KeyCount"`
	MaxKeys        int               `xml:"MaxKeys"`
	IsTruncated    bool              `xml:"IsTruncated"`
	Contents       []S3ObjectContent `xml:"Contents"`
	CommonPrefixes []string          `xml:"CommonPrefixes>Prefix,omitempty"`
}

// S3DeleteObjectReference represents an object key to delete in multi-object delete requests.
type S3DeleteObjectReference struct {
	Key string `xml:"Key"`
}

// DeleteMultipleS3ObjectsInput represents the XML request payload for multi-object deletion.
type DeleteMultipleS3ObjectsInput struct {
	XMLName xml.Name                  `xml:"Delete"`
	Quiet   bool                      `xml:"Quiet"`
	Objects []S3DeleteObjectReference `xml:"Object"`
}

// S3DeletedObjectConfirmation represents a successfully deleted object confirmation.
type S3DeletedObjectConfirmation struct {
	Key string `xml:"Key"`
}

// S3DeleteError represents an error deleting a specific object in multi-object deletion.
type S3DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// DeleteMultipleS3ObjectsResponse represents the XML response payload for multi-object deletion.
type DeleteMultipleS3ObjectsResponse struct {
	XMLName xml.Name                      `xml:"http://s3.amazonaws.com/doc/2006-03-01/ DeleteResult"`
	Deleted []S3DeletedObjectConfirmation `xml:"Deleted,omitempty"`
	Errors  []S3DeleteError               `xml:"Error,omitempty"`
}

// CreateS3MultipartUploadResponse represents the XML response envelope when initiating multipart uploads.
type CreateS3MultipartUploadResponse struct {
	XMLName  xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

// S3CompletedPart represents a single part completion record in multipart complete requests.
type S3CompletedPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// CompleteS3MultipartUploadInput represents the XML request payload to complete a multipart upload.
type CompleteS3MultipartUploadInput struct {
	XMLName xml.Name          `xml:"CompleteMultipartUpload"`
	Parts   []S3CompletedPart `xml:"Part"`
}

// CompleteS3MultipartUploadResponse represents the XML response envelope when completing multipart uploads.
type CompleteS3MultipartUploadResponse struct {
	XMLName  xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// S3ErrorResponse represents standard AWS S3 XML error envelopes.
type S3ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId,omitempty"`
}
