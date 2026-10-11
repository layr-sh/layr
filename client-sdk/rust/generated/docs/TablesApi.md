# \TablesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_tables_create**](TablesApi.md#data_tables_create) | **POST** /v1/_/data/tables | Create a new PostgreSQL table with UUIDv7 primary key
[**data_tables_delete**](TablesApi.md#data_tables_delete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name} | Drop a table and cascade constraints
[**data_tables_get**](TablesApi.md#data_tables_get) | **GET** /v1/_/data/tables/{schema_name}/{table_name} | Inspect table schema, columns, indexes, and constraints
[**data_tables_list**](TablesApi.md#data_tables_list) | **GET** /v1/_/data/tables | List all database tables across configured schemas



## data_tables_create

> models::CreateTableResponse data_tables_create(create_table_input)
Create a new PostgreSQL table with UUIDv7 primary key

Executes DDL to provision a table adhering to Layr conventions (UUIDv7 primary keys, created_at, and last_updated_at timestamps), invalidating catalog caches and emitting data.table.created event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_table_input** | [**CreateTableInput**](CreateTableInput.md) | Request body for data.CreateTableInput | [required] |

### Return type

[**models::CreateTableResponse**](CreateTableResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_delete

> models::DeleteTableResponse data_tables_delete(schema_name, table_name)
Drop a table and cascade constraints

Executes DROP TABLE CASCADE in PostgreSQL, removing the table, associated indexes, triggers, and foreign key constraints. Invalidates catalog caches and emits data.table.deleted event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |

### Return type

[**models::DeleteTableResponse**](DeleteTableResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_get

> models::Table data_tables_get(schema_name, table_name)
Inspect table schema, columns, indexes, and constraints

Retrieves complete architectural schema for a table, detailing columns (types, nullability, defaults), primary/foreign key constraints, indexes, and active RLS policies. Requires data:schema.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |

### Return type

[**models::Table**](Table.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_list

> models::ListTablesResponse data_tables_list()
List all database tables across configured schemas

Queries PostgreSQL information_schema and pg_catalog to return tables, primary key column names, approximate row counts, and RLS enabled statuses across exposed schemas. Requires data:schema.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListTablesResponse**](ListTablesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

