# \TablesIndexesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_tables_indexes_create**](TablesIndexesApi.md#data_tables_indexes_create) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/indexes | Create an index on table columns
[**data_tables_indexes_delete**](TablesIndexesApi.md#data_tables_indexes_delete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name} | Drop an index from a table
[**data_tables_indexes_list**](TablesIndexesApi.md#data_tables_indexes_list) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/indexes | List indexes defined on a table



## data_tables_indexes_create

> models::CreateIndexResponse data_tables_indexes_create(schema_name, table_name, create_index_input)
Create an index on table columns

Executes CREATE INDEX supporting B-Tree, GIN, GiST, or BRIN index methods, unique constraints, and multi-column expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**create_index_input** | [**CreateIndexInput**](CreateIndexInput.md) | Request body for data.CreateIndexInput | [required] |

### Return type

[**models::CreateIndexResponse**](CreateIndexResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_indexes_delete

> models::DeleteIndexResponse data_tables_indexes_delete(schema_name, table_name, index_name)
Drop an index from a table

Executes DROP INDEX in PostgreSQL, removing the index and invalidating query planner metadata. Emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**index_name** | **String** |  | [required] |

### Return type

[**models::DeleteIndexResponse**](DeleteIndexResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_indexes_list

> models::ListIndexesResponse data_tables_indexes_list(schema_name, table_name)
List indexes defined on a table

Queries pg_indexes to return all index definitions, methods, uniqueness constraints, and column lists for the specified table. Requires data:schema.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |

### Return type

[**models::ListIndexesResponse**](ListIndexesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

