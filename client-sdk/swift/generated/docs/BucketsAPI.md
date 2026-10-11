# BucketsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**fileStorageBucketsCreate**](BucketsAPI.md#filestoragebucketscreate) | **POST** /v1/_/file-storage/buckets | Create a storage bucket with backend configuration
[**fileStorageBucketsDelete**](BucketsAPI.md#filestoragebucketsdelete) | **DELETE** /v1/_/file-storage/buckets/{bucket} | Delete a storage bucket and cascade contained objects
[**fileStorageBucketsGet**](BucketsAPI.md#filestoragebucketsget) | **GET** /v1/_/file-storage/buckets/{bucket} | Get storage bucket details by name
[**fileStorageBucketsList**](BucketsAPI.md#filestoragebucketslist) | **GET** /v1/_/file-storage/buckets | List all storage buckets and backend configurations
[**fileStorageBucketsListObjects**](BucketsAPI.md#filestoragebucketslistobjects) | **GET** /v1/_/file-storage/buckets/{bucket}/objects | List objects stored within a bucket
[**fileStorageBucketsUpdate**](BucketsAPI.md#filestoragebucketsupdate) | **PATCH** /v1/_/file-storage/buckets/{bucket} | Update storage bucket configuration and constraints


# **fileStorageBucketsCreate**
```swift
    open class func fileStorageBucketsCreate(createBucketInput: CreateBucketInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Bucket
```

Create a storage bucket with backend configuration

Provisions a new storage bucket in file_storage.buckets with database or S3 backend settings, size limits, allowed MIME types, and public access policy. Emits file_storage.bucket.created event. Requires storage:bucket.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createBucketInput = CreateBucketInput(allowedMimeTypes: ["allowedMimeTypes_example"], backend: "backend_example", backendConfig: "TODO", isPublic: false, maxFileSizeBytes: 123, name: "name_example") // CreateBucketInput | Request body for filestorage.CreateBucketInput

// Create a storage bucket with backend configuration
do {
    let response = try await BucketsAPI.fileStorageBucketsCreate(createBucketInput: createBucketInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createBucketInput** | [**CreateBucketInput**](CreateBucketInput.md) | Request body for filestorage.CreateBucketInput | 

### Return type

[**Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageBucketsDelete**
```swift
    open class func fileStorageBucketsDelete(bucket: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete a storage bucket and cascade contained objects

Deletes a storage bucket from file_storage.buckets and permanently cascades deletion of all stored objects from the backend and database. Emits file_storage.bucket.deleted event. Requires storage:bucket.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 

// Delete a storage bucket and cascade contained objects
do {
    let response = try await BucketsAPI.fileStorageBucketsDelete(bucket: bucket)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageBucketsGet**
```swift
    open class func fileStorageBucketsGet(bucket: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Bucket
```

Get storage bucket details by name

Retrieves configuration, size limits, allowed MIME types, backend type, and public access status for a specific bucket from file_storage.buckets. Requires storage:bucket.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 

// Get storage bucket details by name
do {
    let response = try await BucketsAPI.fileStorageBucketsGet(bucket: bucket)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 

### Return type

[**Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageBucketsList**
```swift
    open class func fileStorageBucketsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListBucketsResponse
```

List all storage buckets and backend configurations

Returns all configured storage buckets from file_storage.buckets, detailing backend type (database or s3), public access flag, file size limits, and allowed MIME types. Requires storage:bucket.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List all storage buckets and backend configurations
do {
    let response = try await BucketsAPI.fileStorageBucketsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListBucketsResponse**](ListBucketsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageBucketsListObjects**
```swift
    open class func fileStorageBucketsListObjects(bucket: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListBucketObjectsResponse
```

List objects stored within a bucket

Queries metadata records in file_storage.objects for the specified bucket, detailing key paths, file sizes, MIME types, ETags, and upload timestamps with pagination. Requires storage:bucket.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 

// List objects stored within a bucket
do {
    let response = try await BucketsAPI.fileStorageBucketsListObjects(bucket: bucket)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 

### Return type

[**ListBucketObjectsResponse**](ListBucketObjectsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageBucketsUpdate**
```swift
    open class func fileStorageBucketsUpdate(bucket: String, updateBucketInput: UpdateBucketInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Bucket
```

Update storage bucket configuration and constraints

Modifies bucket settings in file_storage.buckets including public flag, file size limit, allowed MIME types, and backend configuration. Emits file_storage.bucket.updated event. Requires storage:bucket.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let updateBucketInput = UpdateBucketInput(allowedMimeTypes: ["allowedMimeTypes_example"], backend: "backend_example", backendConfig: "TODO", isPublic: false, maxFileSizeBytes: 123) // UpdateBucketInput | Request body for filestorage.UpdateBucketInput

// Update storage bucket configuration and constraints
do {
    let response = try await BucketsAPI.fileStorageBucketsUpdate(bucket: bucket, updateBucketInput: updateBucketInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **updateBucketInput** | [**UpdateBucketInput**](UpdateBucketInput.md) | Request body for filestorage.UpdateBucketInput | 

### Return type

[**Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

