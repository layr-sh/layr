# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreConfigGet**](ConfigAPI.md#coreconfigget) | **GET** /v1/_/core/config | Get Core runtime configuration
[**coreConfigUpdate**](ConfigAPI.md#coreconfigupdate) | **PUT** /v1/_/core/config | Update Core runtime configuration


# **coreConfigGet**
```swift
    open class func coreConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> RuntimeConfig
```

Get Core runtime configuration

#### Controller:   `layr.sh/core.(*Server).handleGetConfig`  ---  

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get Core runtime configuration
do {
    let response = try await ConfigAPI.coreConfigGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**RuntimeConfig**](RuntimeConfig.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreConfigUpdate**
```swift
    open class func coreConfigUpdate(runtimeConfig: RuntimeConfig, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> RuntimeConfig
```

Update Core runtime configuration

#### Controller:   `layr.sh/core.(*Server).handleUpdateConfig`  ---  

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let runtimeConfig = RuntimeConfig(allowedCidrs: ["allowedCidrs_example"], auditRetentionDays: 123) // RuntimeConfig | Request body for core.RuntimeConfig

// Update Core runtime configuration
do {
    let response = try await ConfigAPI.coreConfigUpdate(runtimeConfig: runtimeConfig)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **runtimeConfig** | [**RuntimeConfig**](RuntimeConfig.md) | Request body for core.RuntimeConfig | 

### Return type

[**RuntimeConfig**](RuntimeConfig.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

