# DLQAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasksDlqList**](DLQAPI.md#tasksdlqlist) | **GET** /v1/_/tasks/dlq | List dead-lettered failed task executions
[**tasksDlqPurge**](DLQAPI.md#tasksdlqpurge) | **DELETE** /v1/_/tasks/dlq/{execution_id} | Permanently purge a failed task from the DLQ
[**tasksDlqRetry**](DLQAPI.md#tasksdlqretry) | **POST** /v1/_/tasks/dlq/{execution_id}/retry | Re-queue a failed task execution from the DLQ


# **tasksDlqList**
```swift
    open class func tasksDlqList(limit: Int? = nil, offset: Int? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListDLQResponse
```

List dead-lettered failed task executions

Queries dead-lettered executions from tasks.executions that exhausted all configured retry attempts, detailing failure errors, stack traces, and attempt counts. Requires tasks:execution.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let limit = 987 // Int | Maximum number of items to return (default 50, maximum 1000) (optional) (default to 50)
let offset = 987 // Int | Number of items to skip before returning results (default 0) (optional) (default to 0)

// List dead-lettered failed task executions
do {
    let response = try await DLQAPI.tasksDlqList(limit: limit, offset: offset)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **Int** | Maximum number of items to return (default 50, maximum 1000) | [optional] [default to 50]
 **offset** | **Int** | Number of items to skip before returning results (default 0) | [optional] [default to 0]

### Return type

[**ListDLQResponse**](ListDLQResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksDlqPurge**
```swift
    open class func tasksDlqPurge(executionId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Permanently purge a failed task from the DLQ

Permanently removes a failed execution record from tasks.executions / dead-letter queue. Requires tasks:execution.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let executionId = "executionId_example" // String | 

// Permanently purge a failed task from the DLQ
do {
    let response = try await DLQAPI.tasksDlqPurge(executionId: executionId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **executionId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksDlqRetry**
```swift
    open class func tasksDlqRetry(executionId: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> RetryDLQResponse
```

Re-queue a failed task execution from the DLQ

Resets a dead-lettered execution in tasks.executions to 'pending' status for immediate worker retry pickup. Requires tasks:execution.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let executionId = "executionId_example" // String | 
let body =  // JSONValue | Request body for core.Empty

// Re-queue a failed task execution from the DLQ
do {
    let response = try await DLQAPI.tasksDlqRetry(executionId: executionId, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **executionId** | **String** |  | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**RetryDLQResponse**](RetryDLQResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

