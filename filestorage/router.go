package filestorage

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all File Storage REST, S3-compatible, and control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	if baseRouter != nil {
		service.registerBaseRoutes(baseRouter)
	}
	if controlPlaneRouter != nil {
		service.registerControlPlaneRoutes(controlPlaneRouter)
	}
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. REST Object Operations
	core.GetRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleDownloadObject,
		core.RouteTag("File Storage REST"),
		core.RouteSummary("Download a file object"),
		core.RouteDescription("Streams binary object content from the configured file storage backend with byte-range support."),
		core.RouteBinaryResponse(http.StatusOK, "File binary stream"),
		core.RouteOperationID("file_storage__objects__download"),
		core.RouteSDKGroupName("filestorage", "objects"),
		core.RouteSDKMethodName("download"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleHeadObject,
		core.RouteTag("File Storage REST"),
		core.RouteSummary("Inspect file object metadata headers"),
		core.RouteDescription("Returns HTTP headers for the object including Content-Length, Content-Type, and ETag."),
		core.RouteOperationID("file_storage__objects__head"),
		core.RouteSDKGroupName("filestorage", "objects"),
		core.RouteSDKMethodName("head"),
	)
	core.PutRoute[Object, core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleUploadObject,
		core.RouteTag("File Storage REST"),
		core.RouteSummary("Upload a file object via PUT"),
		core.RouteDescription("Uploads binary payload into the specified bucket."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__objects__upload_put"),
		core.RouteSDKGroupName("filestorage", "objects"),
		core.RouteSDKMethodName("upload_put"),
	)
	core.PostRoute[Object, core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleUploadObject,
		core.RouteTag("File Storage REST"),
		core.RouteSummary("Upload a file object via POST"),
		core.RouteDescription("Uploads binary payload into the specified bucket."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__objects__upload_post"),
		core.RouteSDKGroupName("filestorage", "objects"),
		core.RouteSDKMethodName("upload_post"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/file-storage/objects/{bucket}/{key...}", service.baseHandler.handleDeleteObject,
		core.RouteTag("File Storage REST"),
		core.RouteSummary("Delete a file object"),
		core.RouteDescription("Permanently removes the object from the file storage backend."),
		core.RouteNoContentResponse("Object deleted"),
		core.RouteOperationID("file_storage__objects__delete"),
		core.RouteSDKGroupName("filestorage", "objects"),
		core.RouteSDKMethodName("delete"),
	)

	// 2. Presigned URL Capability Minting
	core.PostRoute[PresignURLResponse, PresignURLInput](router, "/v1/file-storage/presign", service.baseHandler.handlePresignURL,
		core.RouteTag("File Storage Capability"),
		core.RouteSummary("Mint a presigned capability URL"),
		core.RouteDescription("Generates a cryptographically signed URL allowing temporary read or write access."),
		core.RouteOperationID("file_storage__presign__create"),
		core.RouteSDKGroupName("filestorage", "presign"),
		core.RouteSDKMethodName("create"),
	)

	// 3. S3 Compatibility Gateway
	core.GetRoute[ListS3BucketsResponse](router, "/v1/file-storage/s3", service.baseHandler.handleListS3Buckets,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 ListAllMyBuckets"),
		core.RouteDescription("Returns XML listing of all storage buckets for the authenticated service account."),
		core.RouteOperationID("file_storage__s3__list_buckets"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("list_buckets"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleHeadS3Bucket,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 HeadBucket"),
		core.RouteDescription("Checks if a bucket exists and caller has permission to access it."),
		core.RouteOperationID("file_storage__s3__head_bucket"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("head_bucket"),
	)
	core.GetRoute[ListS3BucketResponse](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleListS3Bucket,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 ListObjectsV2"),
		core.RouteDescription("Lists objects within a bucket using standard S3 ListObjectsV2 XML response."),
		core.RouteOperationID("file_storage__s3__list_bucket"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("list_bucket"),
	)
	core.PostRoute[DeleteMultipleS3ObjectsResponse, DeleteMultipleS3ObjectsInput](router, "/v1/file-storage/s3/{bucket}", service.baseHandler.handleDeleteMultipleS3Objects,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 DeleteMultipleObjects"),
		core.RouteDescription("Deletes multiple objects specified in the XML request payload."),
		core.RouteOperationID("file_storage__s3__delete_multiple"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("delete_multiple"),
	)
	core.GetRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleGetS3Object,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 GetObject"),
		core.RouteDescription("Downloads object stream using AWS S3 protocol with SigV4."),
		core.RouteBinaryResponse(http.StatusOK, "S3 binary stream"),
		core.RouteOperationID("file_storage__s3__get_object"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("get_object"),
	)
	core.HeadRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleHeadS3Object,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 HeadObject"),
		core.RouteDescription("Retrieves object metadata headers using AWS S3 protocol with SigV4."),
		core.RouteOperationID("file_storage__s3__head_object"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("head_object"),
	)
	core.PutRoute[core.Empty, core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handlePutS3Object,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 PutObject or UploadPart"),
		core.RouteDescription("Uploads an object or multipart chunk using AWS S3 protocol with SigV4."),
		core.RouteOperationID("file_storage__s3__put_object"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("put_object"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handlePostS3Object,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 Multipart Initiate or Complete"),
		core.RouteDescription("Initiates or completes an S3 multipart upload session."),
		core.RouteOperationID("file_storage__s3__post_object"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("post_object"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/file-storage/s3/{bucket}/{key...}", service.baseHandler.handleDeleteS3Object,
		core.RouteTag("File Storage S3 Gateway"),
		core.RouteSummary("S3 DeleteObject or AbortMultipartUpload"),
		core.RouteDescription("Deletes an object or aborts a multipart upload using AWS S3 protocol with SigV4."),
		core.RouteNoContentResponse("Deleted"),
		core.RouteOperationID("file_storage__s3__delete_object"),
		core.RouteSDKGroupName("filestorage", "s3"),
		core.RouteSDKMethodName("delete_object"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	core.GetRoute[ListBucketsResponse](router, "/v1/_/file-storage/buckets", service.controlPlaneHandler.handleListBuckets,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("List all storage buckets"),
		core.RouteDescription("Returns all configured storage buckets and their backend configurations."),
		core.RouteOperationID("file_storage__buckets__list"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Bucket, CreateBucketInput](router, "/v1/_/file-storage/buckets", service.controlPlaneHandler.handleCreateBucket,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Create a new storage bucket"),
		core.RouteDescription("Creates a new bucket with either database or S3 backend."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("file_storage__buckets__create"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Bucket](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleGetBucket,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Get bucket details by name"),
		core.RouteDescription("Returns configuration and status for a specific bucket."),
		core.RouteOperationID("file_storage__buckets__get"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("get"),
	)
	core.PatchRoute[Bucket, UpdateBucketInput](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleUpdateBucket,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Update bucket settings"),
		core.RouteDescription("Updates bucket properties such as public flag, backend config, or size limits."),
		core.RouteOperationID("file_storage__buckets__update"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/file-storage/buckets/{bucket}", service.controlPlaneHandler.handleDeleteBucket,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Delete a storage bucket"),
		core.RouteDescription("Deletes a bucket and cascades deletion to all contained objects."),
		core.RouteNoContentResponse("Bucket deleted"),
		core.RouteOperationID("file_storage__buckets__delete"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ListBucketObjectsResponse](router, "/v1/_/file-storage/buckets/{bucket}/objects", service.controlPlaneHandler.handleListBucketObjects,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("List objects in a bucket"),
		core.RouteDescription("Returns metadata of objects stored within a specific bucket."),
		core.RouteOperationID("file_storage__buckets__list_objects"),
		core.RouteSDKGroupName("filestorage", "buckets"),
		core.RouteSDKMethodName("list_objects"),
	)
	core.GetRoute[Config](router, "/v1/_/file-storage/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Get runtime file storage configuration"),
		core.RouteDescription("Returns dynamic runtime settings for the file storage subsystem."),
		core.RouteOperationID("file_storage__config__get"),
		core.RouteSDKGroupName("filestorage", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/file-storage/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("File Storage Control Plane"),
		core.RouteSummary("Update runtime file storage configuration"),
		core.RouteDescription("Updates dynamic runtime settings for the file storage subsystem."),
		core.RouteOperationID("file_storage__config__update"),
		core.RouteSDKGroupName("filestorage", "config"),
		core.RouteSDKMethodName("update"),
	)
}
