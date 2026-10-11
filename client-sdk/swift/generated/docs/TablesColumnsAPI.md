# TablesColumnsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataTablesColumnsCreate**](TablesColumnsAPI.md#datatablescolumnscreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/columns | Add a new column to a table schema
[**dataTablesColumnsDelete**](TablesColumnsAPI.md#datatablescolumnsdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Drop a column from a table schema
[**dataTablesColumnsUpdate**](TablesColumnsAPI.md#datatablescolumnsupdate) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Alter column type, default, nullability, or rename


# **dataTablesColumnsCreate**
```swift
    open class func dataTablesColumnsCreate(schemaName: String, tableName: String, column: Column, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreateColumnResponse
```

Add a new column to a table schema

Executes ALTER TABLE ADD COLUMN with validated PostgreSQL data type, nullability constraint, and optional default expression. Invalidates schema reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let column = Column(defaultValue: "defaultValue_example", isNullable: false, isPrimaryKey: false, isUnique: false, name: "name_example", type: "type_example") // Column | Request body for data.Column

// Add a new column to a table schema
do {
    let response = try await TablesColumnsAPI.dataTablesColumnsCreate(schemaName: schemaName, tableName: tableName, column: column)
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
 **column** | [**Column**](Column.md) | Request body for data.Column | 

### Return type

[**CreateColumnResponse**](CreateColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesColumnsDelete**
```swift
    open class func dataTablesColumnsDelete(schemaName: String, tableName: String, columnName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> DeleteColumnResponse
```

Drop a column from a table schema

Executes ALTER TABLE DROP COLUMN CASCADE, dropping the column and dependent views or indexes. Invalidates reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let columnName = "columnName_example" // String | 

// Drop a column from a table schema
do {
    let response = try await TablesColumnsAPI.dataTablesColumnsDelete(schemaName: schemaName, tableName: tableName, columnName: columnName)
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
 **columnName** | **String** |  | 

### Return type

[**DeleteColumnResponse**](DeleteColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesColumnsUpdate**
```swift
    open class func dataTablesColumnsUpdate(schemaName: String, tableName: String, columnName: String, updateColumnInput: UpdateColumnInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> UpdateColumnResponse
```

Alter column type, default, nullability, or rename

Alters column attributes (data type conversion, set/drop default, set/drop NOT NULL) or renames the column. Invalidates table metadata caches and emits data.table.updated event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let columnName = "columnName_example" // String | 
let updateColumnInput = UpdateColumnInput(defaultValue: "defaultValue_example", isNullable: false, newName: "newName_example", newType: "newType_example") // UpdateColumnInput | Request body for data.UpdateColumnInput

// Alter column type, default, nullability, or rename
do {
    let response = try await TablesColumnsAPI.dataTablesColumnsUpdate(schemaName: schemaName, tableName: tableName, columnName: columnName, updateColumnInput: updateColumnInput)
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
 **columnName** | **String** |  | 
 **updateColumnInput** | [**UpdateColumnInput**](UpdateColumnInput.md) | Request body for data.UpdateColumnInput | 

### Return type

[**UpdateColumnResponse**](UpdateColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

