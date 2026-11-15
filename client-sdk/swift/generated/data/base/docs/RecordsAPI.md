# RecordsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataRecordsCreate**](RecordsAPI.md#datarecordscreate) | **POST** /v1/data/{schema_name}/{table_name} | Insert single or bulk rows with RETURNING *
[**dataRecordsDelete**](RecordsAPI.md#datarecordsdelete) | **DELETE** /v1/data/{schema_name}/{table_name}/{record_id} | Delete a row by primary key
[**dataRecordsGet**](RecordsAPI.md#datarecordsget) | **GET** /v1/data/{schema_name}/{table_name}/{record_id} | Get a single row by primary key
[**dataRecordsList**](RecordsAPI.md#datarecordslist) | **GET** /v1/data/{schema_name}/{table_name} | Query table rows with dynamic filtering, sorting, and pagination
[**dataRecordsPut**](RecordsAPI.md#datarecordsput) | **PUT** /v1/data/{schema_name}/{table_name}/{record_id} | Update entire row by primary key
[**dataRecordsUpdate**](RecordsAPI.md#datarecordsupdate) | **PATCH** /v1/data/{schema_name}/{table_name}/{record_id} | Partially update a row by primary key


# **dataRecordsCreate**
```swift
    open class func dataRecordsCreate(schemaName: String, tableName: String, createRecordInput: CreateRecordInput, onConflict: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreateRecordResponse
```

Insert single or bulk rows with RETURNING *

Inserts a single row or batch of JSON objects into PostgreSQL, automatically applying defaults and returning inserted tuples. Validates against RLS INSERT ... WITH CHECK policies, invalidates table query caches, and emits real-time CDC updates.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let createRecordInput = CreateRecordInput(data: [Record(id: "id_example", properties: "TODO")]) // CreateRecordInput | Request body for data.CreateRecordInput
let onConflict = "onConflict_example" // String | Target column or constraint for upsert on-conflict resolution (optional)

// Insert single or bulk rows with RETURNING *
do {
    let response = try await RecordsAPI.dataRecordsCreate(schemaName: schemaName, tableName: tableName, createRecordInput: createRecordInput, onConflict: onConflict)
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
 **createRecordInput** | [**CreateRecordInput**](CreateRecordInput.md) | Request body for data.CreateRecordInput | 
 **onConflict** | **String** | Target column or constraint for upsert on-conflict resolution | [optional] 

### Return type

[**CreateRecordResponse**](CreateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRecordsDelete**
```swift
    open class func dataRecordsDelete(schemaName: String, tableName: String, recordId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete a row by primary key

Deletes a row by primary key from PostgreSQL subject to caller RLS DELETE ... USING policies, invalidating table query caches and broadcasting CDC events.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let recordId = "recordId_example" // String | 

// Delete a row by primary key
do {
    let response = try await RecordsAPI.dataRecordsDelete(schemaName: schemaName, tableName: tableName, recordId: recordId)
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
 **recordId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRecordsGet**
```swift
    open class func dataRecordsGet(schemaName: String, tableName: String, recordId: String, select: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetRecordResponse
```

Get a single row by primary key

Retrieves a single row from PostgreSQL by UUID or primary key, enforcing caller Row-Level Security (RLS) SELECT policies and executing within caller session claims.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let recordId = "recordId_example" // String | 
let select = "select_example" // String | Column projection (e.g. 'id,name') (optional)

// Get a single row by primary key
do {
    let response = try await RecordsAPI.dataRecordsGet(schemaName: schemaName, tableName: tableName, recordId: recordId, select: select)
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
 **recordId** | **String** |  | 
 **select** | **String** | Column projection (e.g. &#39;id,name&#39;) | [optional] 

### Return type

[**GetRecordResponse**](GetRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRecordsList**
```swift
    open class func dataRecordsList(schemaName: String, tableName: String, limit: Int? = nil, offset: Int? = nil, select: String? = nil, order: String? = nil, cacheTtl: Int? = nil, cacheKeySuffix: String? = nil, cacheKey: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListRecordsResponse
```

Query table rows with dynamic filtering, sorting, and pagination

Queries PostgreSQL table rows with dynamic filter predicates (eq, gte, in, like), JSON path filtering, relational foreign-key embedding, cursor/offset pagination, and multi-column sorting. Injects caller JWT session claims into transaction-local variables for native Row-Level Security (RLS) enforcement. Supports optional KV result caching via ?cache_ttl.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let limit = 987 // Int | Maximum number of items to return (default 50, maximum 1000) (optional) (default to 50)
let offset = 987 // Int | Number of items to skip before returning results (default 0) (optional) (default to 0)
let select = "select_example" // String | Column projection and embedded relations (e.g. 'id,name,posts(*)') (optional)
let order = "order_example" // String | Ordering expression (e.g. 'created_at.desc,id.asc') (optional)
let cacheTtl = 987 // Int | Cache duration in seconds (optional)
let cacheKeySuffix = "cacheKeySuffix_example" // String | Custom suffix for cache key isolation (optional)
let cacheKey = "cacheKey_example" // String | Alias for cache_key_suffix (optional)

// Query table rows with dynamic filtering, sorting, and pagination
do {
    let response = try await RecordsAPI.dataRecordsList(schemaName: schemaName, tableName: tableName, limit: limit, offset: offset, select: select, order: order, cacheTtl: cacheTtl, cacheKeySuffix: cacheKeySuffix, cacheKey: cacheKey)
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
 **limit** | **Int** | Maximum number of items to return (default 50, maximum 1000) | [optional] [default to 50]
 **offset** | **Int** | Number of items to skip before returning results (default 0) | [optional] [default to 0]
 **select** | **String** | Column projection and embedded relations (e.g. &#39;id,name,posts(*)&#39;) | [optional] 
 **order** | **String** | Ordering expression (e.g. &#39;created_at.desc,id.asc&#39;) | [optional] 
 **cacheTtl** | **Int** | Cache duration in seconds | [optional] 
 **cacheKeySuffix** | **String** | Custom suffix for cache key isolation | [optional] 
 **cacheKey** | **String** | Alias for cache_key_suffix | [optional] 

### Return type

[**ListRecordsResponse**](ListRecordsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRecordsPut**
```swift
    open class func dataRecordsPut(schemaName: String, tableName: String, recordId: String, updateRecordInput: UpdateRecordInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> UpdateRecordResponse
```

Update entire row by primary key

Replaces column values for a row identified by primary key, validating against RLS UPDATE policies, invalidating table query caches, and broadcasting CDC changes.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let recordId = "recordId_example" // String | 
let updateRecordInput = UpdateRecordInput(data: Record(id: "id_example", properties: "TODO")) // UpdateRecordInput | Request body for data.UpdateRecordInput

// Update entire row by primary key
do {
    let response = try await RecordsAPI.dataRecordsPut(schemaName: schemaName, tableName: tableName, recordId: recordId, updateRecordInput: updateRecordInput)
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
 **recordId** | **String** |  | 
 **updateRecordInput** | [**UpdateRecordInput**](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | 

### Return type

[**UpdateRecordResponse**](UpdateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataRecordsUpdate**
```swift
    open class func dataRecordsUpdate(schemaName: String, tableName: String, recordId: String, updateRecordInput: UpdateRecordInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> UpdateRecordResponse
```

Partially update a row by primary key

Applies partial column updates to a row identified by primary key, validating against RLS UPDATE ... USING ... WITH CHECK policies, invalidating cached queries, and broadcasting CDC changes.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let schemaName = "schemaName_example" // String | 
let tableName = "tableName_example" // String | 
let recordId = "recordId_example" // String | 
let updateRecordInput = UpdateRecordInput(data: Record(id: "id_example", properties: "TODO")) // UpdateRecordInput | Request body for data.UpdateRecordInput

// Partially update a row by primary key
do {
    let response = try await RecordsAPI.dataRecordsUpdate(schemaName: schemaName, tableName: tableName, recordId: recordId, updateRecordInput: updateRecordInput)
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
 **recordId** | **String** |  | 
 **updateRecordInput** | [**UpdateRecordInput**](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | 

### Return type

[**UpdateRecordResponse**](UpdateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

