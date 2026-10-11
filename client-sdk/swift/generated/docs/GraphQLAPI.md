# GraphQLAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataGraphql**](GraphQLAPI.md#datagraphql) | **POST** /v1/graphql | Execute GraphQL query or mutation


# **dataGraphql**
```swift
    open class func dataGraphql(executeGraphQLInput: ExecuteGraphQLInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ExecuteGraphQLResponse
```

Execute GraphQL query or mutation

Dynamic introspection and execution engine over PostgreSQL tables, supporting complex nested selections, arguments, variables, query depth limits (max depth guard), complexity budgets, and native RLS session context injection.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let executeGraphQLInput = ExecuteGraphQLInput(operationName: "operationName_example", query: "query_example", variables: "TODO") // ExecuteGraphQLInput | Request body for data.ExecuteGraphQLInput

// Execute GraphQL query or mutation
do {
    let response = try await GraphQLAPI.dataGraphql(executeGraphQLInput: executeGraphQLInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **executeGraphQLInput** | [**ExecuteGraphQLInput**](ExecuteGraphQLInput.md) | Request body for data.ExecuteGraphQLInput | 

### Return type

[**ExecuteGraphQLResponse**](ExecuteGraphQLResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

