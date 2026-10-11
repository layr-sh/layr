# TablesIndexesAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataTablesIndexesCreate**](TablesIndexesAPI.md#datatablesindexescreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/indexes | Create an index on table columns
[**dataTablesIndexesDelete**](TablesIndexesAPI.md#datatablesindexesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name} | Drop an index from a table
[**dataTablesIndexesList**](TablesIndexesAPI.md#datatablesindexeslist) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/indexes | List indexes defined on a table


# **dataTablesIndexesCreate**
```swift
    open class func dataTablesIndexesCreate(schemaName: String, tableName: String, createIndexInput: CreateIndexInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreateIndexResponse
```

Create an index on table columns

Executes CREATE INDEX supporting B-Tree, GIN, GiST, or BRIN index methods, unique constraints, and multi-column expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let createIndexInput = CreateIndexInput(columns: ["columns_example"], indexName: "indexName_example", isUnique: false, type: "type_example") // CreateIndexInput | Request body for data.CreateIndexInput

// Create an index on table columns
do {
    let response = try await TablesIndexesAPI.dataTablesIndexesCreate(schemaName: schemaName, tableName: tableName, createIndexInput: createIndexInput)
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
 **createIndexInput** | [**CreateIndexInput**](CreateIndexInput.md) | Request body for data.CreateIndexInput | 

### Return type

[**CreateIndexResponse**](CreateIndexResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesIndexesDelete**
```swift
    open class func dataTablesIndexesDelete(schemaName: String, tableName: String, indexName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> DeleteIndexResponse
```

Drop an index from a table

Executes DROP INDEX in PostgreSQL, removing the index and invalidating query planner metadata. Emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let indexName = "indexName_example" // String | 

// Drop an index from a table
do {
    let response = try await TablesIndexesAPI.dataTablesIndexesDelete(schemaName: schemaName, tableName: tableName, indexName: indexName)
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
 **indexName** | **String** |  | 

### Return type

[**DeleteIndexResponse**](DeleteIndexResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesIndexesList**
```swift
    open class func dataTablesIndexesList(schemaName: String, tableName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListIndexesResponse
```

List indexes defined on a table

Queries pg_indexes to return all index definitions, methods, uniqueness constraints, and column lists for the specified table. Requires data:schema.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 

// List indexes defined on a table
do {
    let response = try await TablesIndexesAPI.dataTablesIndexesList(schemaName: schemaName, tableName: tableName)
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

[**ListIndexesResponse**](ListIndexesResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

