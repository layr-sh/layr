# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionConfigGet**](ConfigAPI.md#functionconfigget) | **GET** /v1/_/function/config | Get runtime function configuration
[**functionConfigUpdate**](ConfigAPI.md#functionconfigupdate) | **PUT** /v1/_/function/config | Update runtime function configuration


# **functionConfigGet**
```swift
    open class func functionConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime function configuration

Returns operational function runtime settings including default isolate timeouts, memory limits, runner concurrency, and isolate pool capacities. Requires function:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime function configuration
do {
    let response = try await ConfigAPI.functionConfigGet()
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

# **functionConfigUpdate**
```swift
    open class func functionConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime function configuration

Modifies operational function runtime settings in function.config, tuning concurrency limits, memory caps, and execution timeouts without restarting the gateway. Requires function:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(allowedEnvironmentKeys: ["allowedEnvironmentKeys_example"], customDomainApexIp: "customDomainApexIp_example", customDomainCnameTarget: "customDomainCnameTarget_example", defaultMemoryLimitMb: 123, defaultRuntime: "defaultRuntime_example", defaultTimeoutSeconds: 123, defaultTlsAcmeEmail: "defaultTlsAcmeEmail_example", maxBundleSizeBytes: 123, workerdRuntime: WorkerdRuntimeConfig(compatibilityDate: "compatibilityDate_example", compatibilityFlags: ["compatibilityFlags_example"])) // Config | Request body for function.Config

// Update runtime function configuration
do {
    let response = try await ConfigAPI.functionConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for function.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

