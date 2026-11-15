# TablesRLSAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataTablesRlsUpdate**](TablesRLSAPI.md#datatablesrlsupdate) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/rls | Toggle Row-Level Security enforcement mode on a table


# **dataTablesRlsUpdate**
```swift
    open class func dataTablesRlsUpdate(schemaName: String, tableName: String, toggleRLSInput: ToggleRLSInput, action: String? = nil, force: Bool? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ToggleRLSResponse
```

Toggle Row-Level Security enforcement mode on a table

Executes ALTER TABLE to ENABLE, DISABLE, or FORCE Row-Level Security (RLS) enforcement on the table. Invalidates table metadata and emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let toggleRLSInput = ToggleRLSInput(action: "action_example", mode: "mode_example") // ToggleRLSInput | Request body for data.ToggleRLSInput
let action = "action_example" // String | RLS enforcement mode ('ENABLE', 'DISABLE', or 'FORCE') (optional)
let force = true // Bool | Whether to enforce RLS for table owners when action is FORCE (optional)

// Toggle Row-Level Security enforcement mode on a table
do {
    let response = try await TablesRLSAPI.dataTablesRlsUpdate(schemaName: schemaName, tableName: tableName, toggleRLSInput: toggleRLSInput, action: action, force: force)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **schemaName** | **String** |  | 
 **tableName** | **String** |  | 
 **toggleRLSInput** | [**ToggleRLSInput**](ToggleRLSInput.md) | Request body for data.ToggleRLSInput | 
 **action** | **String** | RLS enforcement mode (&#39;ENABLE&#39;, &#39;DISABLE&#39;, or &#39;FORCE&#39;) | [optional] 
 **force** | **Bool** | Whether to enforce RLS for table owners when action is FORCE | [optional] 

### Return type

[**ToggleRLSResponse**](ToggleRLSResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

