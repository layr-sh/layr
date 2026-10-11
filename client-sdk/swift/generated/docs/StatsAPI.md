# StatsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionStatsGet**](StatsAPI.md#functionstatsget) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics
[**imageStatsGet**](StatsAPI.md#imagestatsget) | **GET** /v1/_/image/stats | Get image service telemetry statistics
[**tasksStatsGet**](StatsAPI.md#tasksstatsget) | **GET** /v1/_/tasks/stats | Get operational task telemetry and queue statistics


# **functionStatsGet**
```swift
    open class func functionStatsGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetStatsResponse
```

Get function runtime telemetry and worker statistics

Returns real-time runtime statistics including active worker isolates count, memory consumption, runner queue depth, and throughput metrics. Requires function:stats.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get function runtime telemetry and worker statistics
do {
    let response = try await StatsAPI.functionStatsGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imageStatsGet**
```swift
    open class func imageStatsGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetStatsResponse
```

Get image service telemetry statistics

Returns real-time operational statistics including image cache hit ratios, processing latency, memory usage, and transformation throughput. Requires image:stats.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get image service telemetry statistics
do {
    let response = try await StatsAPI.imageStatsGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksStatsGet**
```swift
    open class func tasksStatsGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetStatsResponse
```

Get operational task telemetry and queue statistics

Returns real-time task statistics including total job counts, active queue depth, currently running executions, DLQ size, and execution success rate. Requires tasks:stats.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get operational task telemetry and queue statistics
do {
    let response = try await StatsAPI.tasksStatsGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

