# ObjectsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**fileStorageObjectsDelete**](ObjectsAPI.md#filestorageobjectsdelete) | **DELETE** /v1/file-storage/objects/{bucket}/{key...} | Permanently delete a file object
[**fileStorageObjectsDownload**](ObjectsAPI.md#filestorageobjectsdownload) | **GET** /v1/file-storage/objects/{bucket}/{key...} | Download a file object with byte-range support
[**fileStorageObjectsHead**](ObjectsAPI.md#filestorageobjectshead) | **HEAD** /v1/file-storage/objects/{bucket}/{key...} | Inspect file object metadata headers
[**fileStorageObjectsUploadPost**](ObjectsAPI.md#filestorageobjectsuploadpost) | **POST** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via POST
[**fileStorageObjectsUploadPut**](ObjectsAPI.md#filestorageobjectsuploadput) | **PUT** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via PUT


# **fileStorageObjectsDelete**
```swift
    open class func fileStorageObjectsDelete(bucket: String, key: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Permanently delete a file object

Deletes file object binary payload from the storage backend (database or S3) and removes its metadata row from file_storage.objects. Emits file_storage.object.deleted event.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let key = "key_example" // String |  (might contain slashes)

// Permanently delete a file object
do {
    let response = try await ObjectsAPI.fileStorageObjectsDelete(bucket: bucket, key: key)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **key** | **String** |  (might contain slashes) | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageObjectsDownload**
```swift
    open class func fileStorageObjectsDownload(bucket: String, key: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> URL
```

Download a file object with byte-range support

Streams binary object payload from configured backend (PostgreSQL database bytea or S3) with HTTP Range header support, Content-Type, Content-Length, and ETag validation. Enforces public bucket policy or valid capability credentials.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let key = "key_example" // String |  (might contain slashes)

// Download a file object with byte-range support
do {
    let response = try await ObjectsAPI.fileStorageObjectsDownload(bucket: bucket, key: key)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **key** | **String** |  (might contain slashes) | 

### Return type

**URL**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/octet-stream

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageObjectsHead**
```swift
    open class func fileStorageObjectsHead(bucket: String, key: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue
```

Inspect file object metadata headers

Returns HTTP metadata headers (Content-Length, Content-Type, ETag, Last-Modified) for a stored file without streaming body payload.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let key = "key_example" // String |  (might contain slashes)

// Inspect file object metadata headers
do {
    let response = try await ObjectsAPI.fileStorageObjectsHead(bucket: bucket, key: key)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **key** | **String** |  (might contain slashes) | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageObjectsUploadPost**
```swift
    open class func fileStorageObjectsUploadPost(bucket: String, key: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Object
```

Upload a file object via POST

Uploads binary object payload into the target bucket, enforcing size guards and MIME validation. Stores in backend, writes metadata to file_storage.objects, and emits object created/updated event.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let key = "key_example" // String |  (might contain slashes)
let body =  // JSONValue | Request body for core.Empty

// Upload a file object via POST
do {
    let response = try await ObjectsAPI.fileStorageObjectsUploadPost(bucket: bucket, key: key, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **key** | **String** |  (might contain slashes) | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**Object**](Object.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageObjectsUploadPut**
```swift
    open class func fileStorageObjectsUploadPut(bucket: String, key: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Object
```

Upload a file object via PUT

Stores binary payload in the specified bucket, enforcing bucket file size limits and MIME allowlists. Creates or updates metadata record in file_storage.objects and emits file_storage.object.created or file_storage.object.updated event.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let bucket = "bucket_example" // String | 
let key = "key_example" // String |  (might contain slashes)
let body =  // JSONValue | Request body for core.Empty

// Upload a file object via PUT
do {
    let response = try await ObjectsAPI.fileStorageObjectsUploadPut(bucket: bucket, key: key, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **bucket** | **String** |  | 
 **key** | **String** |  (might contain slashes) | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**Object**](Object.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

