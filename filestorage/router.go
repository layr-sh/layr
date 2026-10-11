package filestorage

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all File Storage REST, S3-compatible, and control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerBaseRoutes(baseRouter)
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. REST Object Operations
	core.GetRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleDownloadObject,
		core.RouteTag("Objects"),
		core.RouteSummary("Download a file object with byte-range support"),
		core.RouteDescription("Streams binary object payload from configured backend (PostgreSQL database bytea or S3) with HTTP Range header support, Content-Type, Content-Length, and ETag validation. Enforces public bucket policy or valid capability credentials."),
		core.RouteBinaryResponse(http.StatusOK, "File binary stream"),
		core.RouteOperationID("file_storage__objects__download"),
		core.RouteSDKGroupName("file_storage", "objects"),
		core.RouteSDKMethodName("download"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleHeadObject,
		core.RouteTag("Objects"),
		core.RouteSummary("Inspect file object metadata headers"),
		core.RouteDescription("Returns HTTP metadata headers (Content-Length, Content-Type, ETag, Last-Modified) for a stored file without streaming body payload."),
		core.RouteOperationID("file_storage__objects__head"),
		core.RouteSDKGroupName("file_storage", "objects"),
		core.RouteSDKMethodName("head"),
	)
	core.PutRoute[Object, core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleUploadObject,
		core.RouteTag("Objects"),
		core.RouteSummary("Upload a file object via PUT"),
		core.RouteDescription("Stores binary payload in the specified bucket, enforcing bucket file size limits and MIME allowlists. Creates or updates metadata record in file_storage.objects and emits file_storage.object.created or file_storage.object.updated event."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__objects__upload_put"),
		core.RouteSDKGroupName("file_storage", "objects"),
		core.RouteSDKMethodName("upload_put"),
	)
	core.PostRoute[Object, core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleUploadObject,
		core.RouteTag("Objects"),
		core.RouteSummary("Upload a file object via POST"),
		core.RouteDescription("Uploads binary object payload into the target bucket, enforcing size guards and MIME validation. Stores in backend, writes metadata to file_storage.objects, and emits object created/updated event."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__objects__upload_post"),
		core.RouteSDKGroupName("file_storage", "objects"),
		core.RouteSDKMethodName("upload_post"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleDeleteObject,
		core.RouteTag("Objects"),
		core.RouteSummary("Permanently delete a file object"),
		core.RouteDescription("Deletes file object binary payload from the storage backend (database or S3) and removes its metadata row from file_storage.objects. Emits file_storage.object.deleted event."),
		core.RouteNoContentResponse("Object deleted"),
		core.RouteOperationID("file_storage__objects__delete"),
		core.RouteSDKGroupName("file_storage", "objects"),
		core.RouteSDKMethodName("delete"),
	)

	// 2. Presigned URL Capability Minting
	core.PostRoute[PresignURLResponse, PresignURLInput](router, "/v1/file-storage/url/presign", service.baseHandler.handlePresignURL,
		core.RouteTag("URLs"),
		core.RouteSummary("Mint a presigned capability URL for temporary file access"),
		core.RouteDescription("Generates a cryptographically signed HMAC-SHA256 capability URL granting temporary read (GET) or write (PUT) access to a specific bucket and key path with configurable expiration TTL."),
		core.RouteOperationID("file_storage__url__presign"),
		core.RouteSDKGroupName("file_storage", "url"),
		core.RouteSDKMethodName("presign"),
	)

	// 3. S3 Compatibility Gateway
	core.GetRoute[ListS3BucketsResponse](router, "/v1/file-storage/s3", service.baseHandler.handleListS3Buckets,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 ListAllMyBuckets"),
		core.RouteDescription("Returns XML listing of all storage buckets for the authenticated service account."),
		core.RouteOperationID("file_storage__s3__list_buckets"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("list_buckets"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleHeadS3Bucket,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 HeadBucket"),
		core.RouteDescription("Checks if a bucket exists and caller has permission to access it."),
		core.RouteOperationID("file_storage__s3__head_bucket"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("head_bucket"),
	)
	core.GetRoute[ListS3BucketResponse](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleListS3Bucket,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 ListObjectsV2"),
		core.RouteDescription("Lists objects within a bucket using standard S3 ListObjectsV2 XML response."),
		core.RouteOperationID("file_storage__s3__list_bucket"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("list_bucket"),
	)
	core.PostRoute[DeleteMultipleS3ObjectsResponse, DeleteMultipleS3ObjectsInput](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleDeleteMultipleS3Objects,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 DeleteMultipleObjects"),
		core.RouteDescription("Deletes multiple objects specified in the XML request payload."),
		core.RouteOperationID("file_storage__s3__delete_multiple"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("delete_multiple"),
	)
	core.GetRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleGetS3Object,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 GetObject"),
		core.RouteDescription("Downloads object stream using AWS S3 protocol with SigV4."),
		core.RouteBinaryResponse(http.StatusOK, "S3 binary stream"),
		core.RouteOperationID("file_storage__s3__get_object"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("get_object"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleHeadS3Object,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 HeadObject"),
		core.RouteDescription("Retrieves object metadata headers using AWS S3 protocol with SigV4."),
		core.RouteOperationID("file_storage__s3__head_object"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("head_object"),
	)
	core.PutRoute[core.Empty, core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handlePutS3Object,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 PutObject or UploadPart"),
		core.RouteDescription("Uploads an object or multipart chunk using AWS S3 protocol with SigV4."),
		core.RouteOperationID("file_storage__s3__put_object"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("put_object"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handlePostS3Object,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 Multipart Initiate or Complete"),
		core.RouteDescription("Initiates or completes an S3 multipart upload session."),
		core.RouteOperationID("file_storage__s3__post_object"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("post_object"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleDeleteS3Object,
		core.RouteHide(),
		core.RouteTag("S3"),
		core.RouteSummary("S3 DeleteObject or AbortMultipartUpload"),
		core.RouteDescription("Deletes an object or aborts a multipart upload using AWS S3 protocol with SigV4."),
		core.RouteNoContentResponse("Deleted"),
		core.RouteOperationID("file_storage__s3__delete_object"),
		core.RouteSDKGroupName("file_storage", "s3"),
		core.RouteSDKMethodName("delete_object"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	core.GetRoute[ListBucketsResponse](router, "/v1/_/file-storage/buckets", service.controlPlaneHandler.handleListBuckets,
		core.RouteTag("Buckets"),
		core.RouteSummary("List all storage buckets and backend configurations"),
		core.RouteDescription("Returns all configured storage buckets from file_storage.buckets, detailing backend type (database or s3), public access flag, file size limits, and allowed MIME types. Requires storage:bucket.read scope."),
		core.RouteOperationID("file_storage__buckets__list"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Bucket, CreateBucketInput](router, "/v1/_/file-storage/buckets", service.controlPlaneHandler.handleCreateBucket,
		core.RouteTag("Buckets"),
		core.RouteSummary("Create a storage bucket with backend configuration"),
		core.RouteDescription("Provisions a new storage bucket in file_storage.buckets with database or S3 backend settings, size limits, allowed MIME types, and public access policy. Emits file_storage.bucket.created event. Requires storage:bucket.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__buckets__create"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Bucket](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleGetBucket,
		core.RouteTag("Buckets"),
		core.RouteSummary("Get storage bucket details by name"),
		core.RouteDescription("Retrieves configuration, size limits, allowed MIME types, backend type, and public access status for a specific bucket from file_storage.buckets. Requires storage:bucket.read scope."),
		core.RouteOperationID("file_storage__buckets__get"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("get"),
	)
	core.PatchRoute[Bucket, UpdateBucketInput](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleUpdateBucket,
		core.RouteTag("Buckets"),
		core.RouteSummary("Update storage bucket configuration and constraints"),
		core.RouteDescription("Modifies bucket settings in file_storage.buckets including public flag, file size limit, allowed MIME types, and backend configuration. Emits file_storage.bucket.updated event. Requires storage:bucket.write scope."),
		core.RouteOperationID("file_storage__buckets__update"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleDeleteBucket,
		core.RouteTag("Buckets"),
		core.RouteSummary("Delete a storage bucket and cascade contained objects"),
		core.RouteDescription("Deletes a storage bucket from file_storage.buckets and permanently cascades deletion of all stored objects from the backend and database. Emits file_storage.bucket.deleted event. Requires storage:bucket.write scope."),
		core.RouteNoContentResponse("Bucket deleted"),
		core.RouteOperationID("file_storage__buckets__delete"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ListBucketObjectsResponse](router, "/v1/_/file-storage/buckets/{bucket}/objects", service.controlPlaneHandler.handleListBucketObjects,
		core.RouteTag("Buckets"),
		core.RouteSummary("List objects stored within a bucket"),
		core.RouteDescription("Queries metadata records in file_storage.objects for the specified bucket, detailing key paths, file sizes, MIME types, ETags, and upload timestamps with pagination. Requires storage:bucket.read scope."),
		core.RouteOperationID("file_storage__buckets__list_objects"),
		core.RouteSDKGroupName("file_storage", "buckets"),
		core.RouteSDKMethodName("list_objects"),
	)
	core.GetRoute[Config](router, "/v1/_/file-storage/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Get runtime file storage configuration"),
		core.RouteDescription("Returns dynamic operational configuration settings for the file storage subsystem including global size limits and default backends. Requires storage:config.read scope."),
		core.RouteOperationID("file_storage__config__get"),
		core.RouteSDKGroupName("file_storage", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/file-storage/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update runtime file storage configuration"),
		core.RouteDescription("Updates dynamic operational configuration for the file storage subsystem in storage.config, tuning global upload limits and backends without restart. Requires storage:config.write scope."),
		core.RouteOperationID("file_storage__config__update"),
		core.RouteSDKGroupName("file_storage", "config"),
		core.RouteSDKMethodName("update"),
	)
}
