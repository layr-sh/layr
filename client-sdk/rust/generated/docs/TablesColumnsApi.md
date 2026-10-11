# \TablesColumnsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_tables_columns_create**](TablesColumnsApi.md#data_tables_columns_create) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/columns | Add a new column to a table schema
[**data_tables_columns_delete**](TablesColumnsApi.md#data_tables_columns_delete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Drop a column from a table schema
[**data_tables_columns_update**](TablesColumnsApi.md#data_tables_columns_update) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Alter column type, default, nullability, or rename



## data_tables_columns_create

> models::CreateColumnResponse data_tables_columns_create(schema_name, table_name, column)
Add a new column to a table schema

Executes ALTER TABLE ADD COLUMN with validated PostgreSQL data type, nullability constraint, and optional default expression. Invalidates schema reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**column** | [**Column**](Column.md) | Request body for data.Column | [required] |

### Return type

[**models::CreateColumnResponse**](CreateColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_columns_delete

> models::DeleteColumnResponse data_tables_columns_delete(schema_name, table_name, column_name)
Drop a column from a table schema

Executes ALTER TABLE DROP COLUMN CASCADE, dropping the column and dependent views or indexes. Invalidates reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**column_name** | **String** |  | [required] |

### Return type

[**models::DeleteColumnResponse**](DeleteColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_columns_update

> models::UpdateColumnResponse data_tables_columns_update(schema_name, table_name, column_name, update_column_input)
Alter column type, default, nullability, or rename

Alters column attributes (data type conversion, set/drop default, set/drop NOT NULL) or renames the column. Invalidates table metadata caches and emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**column_name** | **String** |  | [required] |
**update_column_input** | [**UpdateColumnInput**](UpdateColumnInput.md) | Request body for data.UpdateColumnInput | [required] |

### Return type

[**models::UpdateColumnResponse**](UpdateColumnResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

