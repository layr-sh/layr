# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasksConfigGet**](ConfigAPI.md#tasksconfigget) | **GET** /v1/_/tasks/config | Get runtime tasks orchestrator configuration
[**tasksConfigUpdate**](ConfigAPI.md#tasksconfigupdate) | **PUT** /v1/_/tasks/config | Update runtime tasks orchestrator configuration


# **tasksConfigGet**
```swift
    open class func tasksConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime tasks orchestrator configuration

Returns operational task configuration settings including worker concurrency limits, poller interval, default timeout guards, and exponential retry backoff parameters. Requires tasks:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime tasks orchestrator configuration
do {
    let response = try await ConfigAPI.tasksConfigGet()
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

# **tasksConfigUpdate**
```swift
    open class func tasksConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime tasks orchestrator configuration

Updates operational task configuration in tasks.config, dynamically adjusting worker concurrency, polling intervals, and retry backoff limits without restarting workers. Requires tasks:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(concurrencyLimit: 123, pollIntervalMs: 123, retryInitialDelaySeconds: 123, retryMaxAttempts: 123, retryMaxDelaySeconds: 123, timeoutSeconds: 123) // Config | Request body for tasks.Config

// Update runtime tasks orchestrator configuration
do {
    let response = try await ConfigAPI.tasksConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for tasks.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

