# \RecordsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_records_create**](RecordsApi.md#data_records_create) | **POST** /v1/data/{schema_name}/{table_name} | Insert single or bulk rows with RETURNING *
[**data_records_delete**](RecordsApi.md#data_records_delete) | **DELETE** /v1/data/{schema_name}/{table_name}/{record_id} | Delete a row by primary key
[**data_records_get**](RecordsApi.md#data_records_get) | **GET** /v1/data/{schema_name}/{table_name}/{record_id} | Get a single row by primary key
[**data_records_list**](RecordsApi.md#data_records_list) | **GET** /v1/data/{schema_name}/{table_name} | Query table rows with dynamic filtering, sorting, and pagination
[**data_records_put**](RecordsApi.md#data_records_put) | **PUT** /v1/data/{schema_name}/{table_name}/{record_id} | Update entire row by primary key
[**data_records_update**](RecordsApi.md#data_records_update) | **PATCH** /v1/data/{schema_name}/{table_name}/{record_id} | Partially update a row by primary key



## data_records_create

> models::CreateRecordResponse data_records_create(schema_name, table_name, create_record_input)
Insert single or bulk rows with RETURNING *

Inserts a single row or batch of JSON objects into PostgreSQL, automatically applying defaults and returning inserted tuples. Validates against RLS INSERT ... WITH CHECK policies, invalidates table query caches, and emits real-time CDC updates.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**create_record_input** | [**CreateRecordInput**](CreateRecordInput.md) | Request body for data.CreateRecordInput | [required] |

### Return type

[**models::CreateRecordResponse**](CreateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_records_delete

> serde_json::Value data_records_delete(schema_name, table_name, record_id)
Delete a row by primary key

Deletes a row by primary key from PostgreSQL subject to caller RLS DELETE ... USING policies, invalidating table query caches and broadcasting CDC events.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**record_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_records_get

> models::GetRecordResponse data_records_get(schema_name, table_name, record_id)
Get a single row by primary key

Retrieves a single row from PostgreSQL by UUID or primary key, enforcing caller Row-Level Security (RLS) SELECT policies and executing within caller session claims.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**record_id** | **String** |  | [required] |

### Return type

[**models::GetRecordResponse**](GetRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_records_list

> models::ListRecordsResponse data_records_list(schema_name, table_name)
Query table rows with dynamic filtering, sorting, and pagination

Queries PostgreSQL table rows with dynamic filter predicates (eq, gte, in, like), JSON path filtering, relational foreign-key embedding, cursor/offset pagination, and multi-column sorting. Injects caller JWT session claims into transaction-local variables for native Row-Level Security (RLS) enforcement. Supports optional KV result caching via ?cache_ttl.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |

### Return type

[**models::ListRecordsResponse**](ListRecordsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_records_put

> models::UpdateRecordResponse data_records_put(schema_name, table_name, record_id, update_record_input)
Update entire row by primary key

Replaces column values for a row identified by primary key, validating against RLS UPDATE policies, invalidating table query caches, and broadcasting CDC changes.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**record_id** | **String** |  | [required] |
**update_record_input** | [**UpdateRecordInput**](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | [required] |

### Return type

[**models::UpdateRecordResponse**](UpdateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_records_update

> models::UpdateRecordResponse data_records_update(schema_name, table_name, record_id, update_record_input)
Partially update a row by primary key

Applies partial column updates to a row identified by primary key, validating against RLS UPDATE ... USING ... WITH CHECK policies, invalidating cached queries, and broadcasting CDC changes.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**record_id** | **String** |  | [required] |
**update_record_input** | [**UpdateRecordInput**](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | [required] |

### Return type

[**models::UpdateRecordResponse**](UpdateRecordResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

