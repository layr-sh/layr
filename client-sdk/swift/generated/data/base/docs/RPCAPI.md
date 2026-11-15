# RPCAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataRpcMutation**](RPCAPI.md#datarpcmutation) | **POST** /v1/data/{schema_name}/rpc/{function_name} | Execute database stored function or procedure with JSON payload
[**dataRpcQuery**](RPCAPI.md#datarpcquery) | **GET** /v1/data/{schema_name}/rpc/{function_name} | Execute read-only database stored function or procedure


# **dataRpcMutation**
```swift
    open class func dataRpcMutation(schemaName: String, functionName: String, body: JSONValue, invalidateTables: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue
```

Execute database stored function or procedure with JSON payload

Invokes a PostgreSQL stored procedure or function with JSON request body arguments, setting caller RLS session claims and returning execution results.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let functionName = "functionName_example" // String | 
let body = "TODO" // JSONValue | Request body for data.ExecuteFunctionInput
let invalidateTables = "invalidateTables_example" // String | Comma-separated table names whose cache should be invalidated (optional)

// Execute database stored function or procedure with JSON payload
do {
    let response = try await RPCAPI.dataRpcMutation(schemaName: schemaName, functionName: functionName, body: body, invalidateTables: invalidateTables)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **schemaName** | **String** |  | 
 **functionName** | **String** |  | 
 **body** | **JSONValue** | Request body for data.ExecuteFunctionInput | 
 **invalidateTables** | **String** | Comma-separated table names whose cache should be invalidated | [optional] 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRpcQuery**
```swift
    open class func dataRpcQuery(schemaName: String, functionName: String, invalidateTables: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue
```

Execute read-only database stored function or procedure

Invokes a PostgreSQL stored function or procedure with arguments passed via query parameters, setting transaction-local caller RLS session claims.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let functionName = "functionName_example" // String | 
let invalidateTables = "invalidateTables_example" // String | Comma-separated table names whose cache should be invalidated (optional)

// Execute read-only database stored function or procedure
do {
    let response = try await RPCAPI.dataRpcQuery(schemaName: schemaName, functionName: functionName, invalidateTables: invalidateTables)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **schemaName** | **String** |  | 
 **functionName** | **String** |  | 
 **invalidateTables** | **String** | Comma-separated table names whose cache should be invalidated | [optional] 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

