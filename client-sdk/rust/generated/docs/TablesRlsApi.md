# \TablesRlsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_tables_rls_update**](TablesRlsApi.md#data_tables_rls_update) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/rls | Toggle Row-Level Security enforcement mode on a table



## data_tables_rls_update

> models::ToggleRlsResponse data_tables_rls_update(schema_name, table_name, toggle_rls_input, action, force)
Toggle Row-Level Security enforcement mode on a table

Executes ALTER TABLE to ENABLE, DISABLE, or FORCE Row-Level Security (RLS) enforcement on the table. Invalidates table metadata and emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**toggle_rls_input** | [**ToggleRlsInput**](ToggleRlsInput.md) | Request body for data.ToggleRLSInput | [required] |
**action** | Option<**String**> | RLS enforcement mode ('ENABLE', 'DISABLE', or 'FORCE') |  |
**force** | Option<**bool**> | Whether to enforce RLS for table owners when action is FORCE |  |

### Return type

[**models::ToggleRlsResponse**](ToggleRLSResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

