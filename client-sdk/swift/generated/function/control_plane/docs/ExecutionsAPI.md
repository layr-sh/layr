# ExecutionsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionExecutionsList**](ExecutionsAPI.md#functionexecutionslist) | **GET** /v1/_/function/executions | Query execution history logs across all endpoints


# **functionExecutionsList**
```swift
    open class func functionExecutionsList(limit: Int? = nil, offset: Int? = nil, endpointId: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListExecutionsResponse
```

Query execution history logs across all endpoints

Queries execution telemetry logs from function.executions across all endpoints, returning status codes, execution duration in milliseconds, memory utilization, and stdout/stderr streams. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let limit = 987 // Int | Maximum number of items to return (default 50, maximum 1000) (optional) (default to 50)
let offset = 987 // Int | Number of items to skip before returning results (default 0) (optional) (default to 0)
let endpointId = "endpointId_example" // String | Filter executions by endpoint UUID (optional)

// Query execution history logs across all endpoints
do {
    let response = try await ExecutionsAPI.functionExecutionsList(limit: limit, offset: offset, endpointId: endpointId)
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
 **endpointId** | **String** | Filter executions by endpoint UUID | [optional] 

### Return type

[**ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

