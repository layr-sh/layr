# \RpcApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_rpc_mutation**](RpcApi.md#data_rpc_mutation) | **POST** /v1/data/{schema_name}/rpc/{function_name} | Execute database stored function or procedure with JSON payload
[**data_rpc_query**](RpcApi.md#data_rpc_query) | **GET** /v1/data/{schema_name}/rpc/{function_name} | Execute read-only database stored function or procedure



## data_rpc_mutation

> serde_json::Value data_rpc_mutation(schema_name, function_name, body, invalidate_tables)
Execute database stored function or procedure with JSON payload

Invokes a PostgreSQL stored procedure or function with JSON request body arguments, setting caller RLS session claims and returning execution results.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**function_name** | **String** |  | [required] |
**body** | **serde_json::Value** | Request body for data.ExecuteFunctionInput | [required] |
**invalidate_tables** | Option<**String**> | Comma-separated table names whose cache should be invalidated |  |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_rpc_query

> serde_json::Value data_rpc_query(schema_name, function_name, invalidate_tables)
Execute read-only database stored function or procedure

Invokes a PostgreSQL stored function or procedure with arguments passed via query parameters, setting transaction-local caller RLS session claims.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**function_name** | **String** |  | [required] |
**invalidate_tables** | Option<**String**> | Comma-separated table names whose cache should be invalidated |  |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

