# EndpointsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionEndpointsCreate**](EndpointsAPI.md#functionendpointscreate) | **POST** /v1/_/function/endpoints | Register a new serverless endpoint
[**functionEndpointsDelete**](EndpointsAPI.md#functionendpointsdelete) | **DELETE** /v1/_/function/endpoints/{endpoint_id} | Delete serverless endpoint and unload runtime isolates
[**functionEndpointsExecutions**](EndpointsAPI.md#functionendpointsexecutions) | **GET** /v1/_/function/endpoints/{endpoint_id}/executions | Query execution history logs for a specific endpoint
[**functionEndpointsGet**](EndpointsAPI.md#functionendpointsget) | **GET** /v1/_/function/endpoints/{endpoint_id} | Get serverless endpoint definition by ID
[**functionEndpointsList**](EndpointsAPI.md#functionendpointslist) | **GET** /v1/_/function/endpoints | List all registered serverless endpoints
[**functionEndpointsUpdate**](EndpointsAPI.md#functionendpointsupdate) | **PUT** /v1/_/function/endpoints/{endpoint_id} | Update serverless endpoint definition


# **functionEndpointsCreate**
```swift
    open class func functionEndpointsCreate(createEndpointInput: CreateEndpointInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Endpoint
```

Register a new serverless endpoint

Provisions a new serverless endpoint definition in function.endpoints with route path, name, runtime environment (Node, Deno, Python, Go), timeout guards, memory limits, and environment variables. Emits function.endpoint.created event. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createEndpointInput = CreateEndpointInput(description: "description_example", entrypoint: "entrypoint_example", isPublic: false, memoryLimitMb: 123, name: "name_example", runtime: "runtime_example", timeoutSeconds: 123) // CreateEndpointInput | Request body for function.CreateEndpointInput

// Register a new serverless endpoint
do {
    let response = try await EndpointsAPI.functionEndpointsCreate(createEndpointInput: createEndpointInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createEndpointInput** | [**CreateEndpointInput**](CreateEndpointInput.md) | Request body for function.CreateEndpointInput | 

### Return type

[**Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointsDelete**
```swift
    open class func functionEndpointsDelete(endpointId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete serverless endpoint and unload runtime isolates

Permanently deletes an endpoint from function.endpoints, unloads running worker isolates, cascades historical deployments, and removes custom domain routes. Emits function.endpoint.deleted event. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 

// Delete serverless endpoint and unload runtime isolates
do {
    let response = try await EndpointsAPI.functionEndpointsDelete(endpointId: endpointId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointsExecutions**
```swift
    open class func functionEndpointsExecutions(endpointId: String, limit: Int? = nil, offset: Int? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListExecutionsResponse
```

Query execution history logs for a specific endpoint

Returns historical invocation audit records from function.executions for a specific endpoint, detailing response codes, durations, memory usage, and execution log streams. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let limit = 987 // Int | Maximum number of items to return (default 50, maximum 1000) (optional) (default to 50)
let offset = 987 // Int | Number of items to skip before returning results (default 0) (optional) (default to 0)

// Query execution history logs for a specific endpoint
do {
    let response = try await EndpointsAPI.functionEndpointsExecutions(endpointId: endpointId, limit: limit, offset: offset)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **limit** | **Int** | Maximum number of items to return (default 50, maximum 1000) | [optional] [default to 50]
 **offset** | **Int** | Number of items to skip before returning results (default 0) | [optional] [default to 0]

### Return type

[**ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointsGet**
```swift
    open class func functionEndpointsGet(endpointId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Endpoint
```

Get serverless endpoint definition by ID

Retrieves complete definition of a serverless endpoint from function.endpoints by ID or name, including runtime configuration, environment variables, and active deployment version. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 

// Get serverless endpoint definition by ID
do {
    let response = try await EndpointsAPI.functionEndpointsGet(endpointId: endpointId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 

### Return type

[**Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointsList**
```swift
    open class func functionEndpointsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListEndpointsResponse
```

List all registered serverless endpoints

Returns all registered serverless endpoints from function.endpoints with route names, runtime engines, timeout guards, memory limits, active deployment IDs, and status. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List all registered serverless endpoints
do {
    let response = try await EndpointsAPI.functionEndpointsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListEndpointsResponse**](ListEndpointsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointsUpdate**
```swift
    open class func functionEndpointsUpdate(endpointId: String, updateEndpointInput: UpdateEndpointInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Endpoint
```

Update serverless endpoint definition

Modifies an existing serverless endpoint definition in function.endpoints, updating environment variables, timeout guards, or memory limits. Emits function.endpoint.updated event. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let updateEndpointInput = UpdateEndpointInput(description: "description_example", entrypoint: "entrypoint_example", isPublic: false, memoryLimitMb: 123, runtime: "runtime_example", timeoutSeconds: 123) // UpdateEndpointInput | Request body for function.UpdateEndpointInput

// Update serverless endpoint definition
do {
    let response = try await EndpointsAPI.functionEndpointsUpdate(endpointId: endpointId, updateEndpointInput: updateEndpointInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **updateEndpointInput** | [**UpdateEndpointInput**](UpdateEndpointInput.md) | Request body for function.UpdateEndpointInput | 

### Return type

[**Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

