# TablesPoliciesAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataTablesPoliciesCreate**](TablesPoliciesAPI.md#datatablespoliciescreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/policies | Create a Row-Level Security policy on a table
[**dataTablesPoliciesDelete**](TablesPoliciesAPI.md#datatablespoliciesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name} | Drop an RLS policy from a table
[**dataTablesPoliciesList**](TablesPoliciesAPI.md#datatablespolicieslist) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/policies | List Row-Level Security policies on a table


# **dataTablesPoliciesCreate**
```swift
    open class func dataTablesPoliciesCreate(schemaName: String, tableName: String, createPolicyInput: CreatePolicyInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreatePolicyResponse
```

Create a Row-Level Security policy on a table

Executes CREATE POLICY in PostgreSQL, attaching declarative row-level access control rules for specific database roles and SQL commands with USING and WITH CHECK expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let createPolicyInput = CreatePolicyInput(command: "command_example", name: "name_example", permissive: "permissive_example", roles: ["roles_example"], usingExpression: "usingExpression_example", withCheckExpression: "withCheckExpression_example") // CreatePolicyInput | Request body for data.CreatePolicyInput

// Create a Row-Level Security policy on a table
do {
    let response = try await TablesPoliciesAPI.dataTablesPoliciesCreate(schemaName: schemaName, tableName: tableName, createPolicyInput: createPolicyInput)
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
 **createPolicyInput** | [**CreatePolicyInput**](CreatePolicyInput.md) | Request body for data.CreatePolicyInput | 

### Return type

[**CreatePolicyResponse**](CreatePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesPoliciesDelete**
```swift
    open class func dataTablesPoliciesDelete(schemaName: String, tableName: String, policyName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> DeletePolicyResponse
```

Drop an RLS policy from a table

Executes DROP POLICY on the specified table, removing the security policy from PostgreSQL enforcement. Emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let policyName = "policyName_example" // String | 

// Drop an RLS policy from a table
do {
    let response = try await TablesPoliciesAPI.dataTablesPoliciesDelete(schemaName: schemaName, tableName: tableName, policyName: policyName)
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
 **policyName** | **String** |  | 

### Return type

[**DeletePolicyResponse**](DeletePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesPoliciesList**
```swift
    open class func dataTablesPoliciesList(schemaName: String, tableName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListPoliciesResponse
```

List Row-Level Security policies on a table

Queries pg_policies to list active Row-Level Security (RLS) policies, detailing command targets (SELECT, INSERT, UPDATE, DELETE), roles, USING filters, and WITH CHECK expressions. Requires data:schema.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 

// List Row-Level Security policies on a table
do {
    let response = try await TablesPoliciesAPI.dataTablesPoliciesList(schemaName: schemaName, tableName: tableName)
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

### Return type

[**ListPoliciesResponse**](ListPoliciesResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

