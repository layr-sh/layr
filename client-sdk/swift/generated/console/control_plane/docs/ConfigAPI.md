# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**consoleConfigGet**](ConfigAPI.md#consoleconfigget) | **GET** /v1/_/console/config | Get dynamic runtime console configuration
[**consoleConfigUpdate**](ConfigAPI.md#consoleconfigupdate) | **PUT** /v1/_/console/config | Update dynamic runtime console configuration


# **consoleConfigGet**
```swift
    open class func consoleConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get dynamic runtime console configuration

Returns active runtime console configuration settings including session duration, MFA enforcement policy, and allowed origins. Requires console:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get dynamic runtime console configuration
do {
    let response = try await ConfigAPI.consoleConfigGet()
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

# **consoleConfigUpdate**
```swift
    open class func consoleConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update dynamic runtime console configuration

Mutates runtime console configuration parameters in console.config, validating structure and applying settings dynamically across cluster nodes. Requires console:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(mfaEnforced: false, sessionIdleTimeoutSeconds: 123, sessionMaxLifetimeSeconds: 123) // Config | Request body for console.Config

// Update dynamic runtime console configuration
do {
    let response = try await ConfigAPI.consoleConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for console.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

