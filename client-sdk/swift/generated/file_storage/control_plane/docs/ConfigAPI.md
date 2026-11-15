# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**fileStorageConfigGet**](ConfigAPI.md#filestorageconfigget) | **GET** /v1/_/file-storage/config | Get runtime file storage configuration
[**fileStorageConfigUpdate**](ConfigAPI.md#filestorageconfigupdate) | **PUT** /v1/_/file-storage/config | Update runtime file storage configuration


# **fileStorageConfigGet**
```swift
    open class func fileStorageConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime file storage configuration

Returns dynamic operational configuration settings for the file storage subsystem including global size limits and default backends. Requires storage:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime file storage configuration
do {
    let response = try await ConfigAPI.fileStorageConfigGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **fileStorageConfigUpdate**
```swift
    open class func fileStorageConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime file storage configuration

Updates dynamic operational configuration for the file storage subsystem in storage.config, tuning global upload limits and backends without restart. Requires storage:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(chunkSizeBytes: 123, defaultAllowedMimeTypes: ["defaultAllowedMimeTypes_example"], defaultMaxFileSizeBytes: 123, enabled: false, presignTokenExpirySeconds: 123) // Config | Request body for filestorage.Config

// Update runtime file storage configuration
do {
    let response = try await ConfigAPI.fileStorageConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for filestorage.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

