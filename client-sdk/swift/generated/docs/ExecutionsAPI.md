# ExecutionsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionExecutionsList**](ExecutionsAPI.md#functionexecutionslist) | **GET** /v1/_/function/executions | Query execution history logs across all endpoints
[**tasksExecutionsCreate**](ExecutionsAPI.md#tasksexecutionscreate) | **POST** /v1/_/tasks/executions | Trigger an immediate out-of-schedule execution
[**tasksExecutionsList**](ExecutionsAPI.md#tasksexecutionslist) | **GET** /v1/_/tasks/executions | Query execution audit logs


# **functionExecutionsList**
```swift
    open class func functionExecutionsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListExecutionsResponse
```

Query execution history logs across all endpoints

Queries execution telemetry logs from function.executions across all endpoints, returning status codes, execution duration in milliseconds, memory utilization, and stdout/stderr streams. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Query execution history logs across all endpoints
do {
    let response = try await ExecutionsAPI.functionExecutionsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksExecutionsCreate**
```swift
    open class func tasksExecutionsCreate(triggerExecutionInput: TriggerExecutionInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> TriggerExecutionResponse
```

Trigger an immediate out-of-schedule execution

Manually enqueues an execution for a job in tasks.executions with status 'pending' for immediate worker dispatch out-of-schedule. Requires tasks:execution.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let triggerExecutionInput = TriggerExecutionInput(jobId: 123, payload: 123) // TriggerExecutionInput | Request body for tasks.TriggerExecutionInput

// Trigger an immediate out-of-schedule execution
do {
    let response = try await ExecutionsAPI.tasksExecutionsCreate(triggerExecutionInput: triggerExecutionInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **triggerExecutionInput** | [**TriggerExecutionInput**](TriggerExecutionInput.md) | Request body for tasks.TriggerExecutionInput | 

### Return type

[**TriggerExecutionResponse**](TriggerExecutionResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksExecutionsList**
```swift
    open class func tasksExecutionsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListExecutionsResponse
```

Query execution audit logs

Queries execution history records from tasks.executions with filtering by job ID, execution status (pending, running, completed, failed, dead_lettered), duration, and date ranges. Requires tasks:execution.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Query execution audit logs
do {
    let response = try await ExecutionsAPI.tasksExecutionsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

