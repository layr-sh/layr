# StatsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**fileStorageStatsGet**](StatsAPI.md#filestoragestatsget) | **GET** /v1/_/file-storage/stats | Get file storage usage statistics


# **fileStorageStatsGet**
```swift
    open class func fileStorageStatsGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> StatsResponse
```

Get file storage usage statistics

Reports bucket counts, tracked object counts and logical bytes by backend, database chunk payload bytes, pending multipart usage, per-bucket usage and content type distribution. S3 usage includes only locally registered objects, not untracked upstream contents. Requires storage:bucket.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get file storage usage statistics
do {
    let response = try await StatsAPI.fileStorageStatsGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**StatsResponse**](StatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

