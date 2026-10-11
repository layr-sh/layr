# TablesAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataTablesCreate**](TablesAPI.md#datatablescreate) | **POST** /v1/_/data/tables | Create a new PostgreSQL table with UUIDv7 primary key
[**dataTablesDelete**](TablesAPI.md#datatablesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name} | Drop a table and cascade constraints
[**dataTablesGet**](TablesAPI.md#datatablesget) | **GET** /v1/_/data/tables/{schema_name}/{table_name} | Inspect table schema, columns, indexes, and constraints
[**dataTablesList**](TablesAPI.md#datatableslist) | **GET** /v1/_/data/tables | List all database tables across configured schemas


# **dataTablesCreate**
```swift
    open class func dataTablesCreate(createTableInput: CreateTableInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreateTableResponse
```

Create a new PostgreSQL table with UUIDv7 primary key

Executes DDL to provision a table adhering to Layr conventions (UUIDv7 primary keys, created_at, and last_updated_at timestamps), invalidating catalog caches and emitting data.table.created event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createTableInput = CreateTableInput(columns: [Column(defaultValue: "defaultValue_example", isNullable: false, isPrimaryKey: false, isUnique: false, name: "name_example", type: "type_example")], foreignKeys: [ForeignKey(column: "column_example", constraintName: "constraintName_example", foreignColumn: "foreignColumn_example", foreignSchema: "foreignSchema_example", foreignTable: "foreignTable_example", onDelete: "onDelete_example", onUpdate: "onUpdate_example")], name: "name_example", primaryKeyName: "primaryKeyName_example", schema: "schema_example") // CreateTableInput | Request body for data.CreateTableInput

// Create a new PostgreSQL table with UUIDv7 primary key
do {
    let response = try await TablesAPI.dataTablesCreate(createTableInput: createTableInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createTableInput** | [**CreateTableInput**](CreateTableInput.md) | Request body for data.CreateTableInput | 

### Return type

[**CreateTableResponse**](CreateTableResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesDelete**
```swift
    open class func dataTablesDelete(schemaName: String, tableName: String, cascade: Bool? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> DeleteTableResponse
```

Drop a table and cascade constraints

Executes DROP TABLE CASCADE in PostgreSQL, removing the table, associated indexes, triggers, and foreign key constraints. Invalidates catalog caches and emits data.table.deleted event. Requires data:schema.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let cascade = true // Bool | Cascade table drop to dependent constraints and views (optional)

// Drop a table and cascade constraints
do {
    let response = try await TablesAPI.dataTablesDelete(schemaName: schemaName, tableName: tableName, cascade: cascade)
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
 **cascade** | **Bool** | Cascade table drop to dependent constraints and views | [optional] 

### Return type

[**DeleteTableResponse**](DeleteTableResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesGet**
```swift
    open class func dataTablesGet(schemaName: String, tableName: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Table
```

Inspect table schema, columns, indexes, and constraints

Retrieves complete architectural schema for a table, detailing columns (types, nullability, defaults), primary/foreign key constraints, indexes, and active RLS policies. Requires data:schema.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 

// Inspect table schema, columns, indexes, and constraints
do {
    let response = try await TablesAPI.dataTablesGet(schemaName: schemaName, tableName: tableName)
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

[**Table**](Table.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataTablesList**
```swift
    open class func dataTablesList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListTablesResponse
```

List all database tables across configured schemas

Queries PostgreSQL information_schema and pg_catalog to return tables, primary key column names, approximate row counts, and RLS enabled statuses across exposed schemas. Requires data:schema.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List all database tables across configured schemas
do {
    let response = try await TablesAPI.dataTablesList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListTablesResponse**](ListTablesResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

