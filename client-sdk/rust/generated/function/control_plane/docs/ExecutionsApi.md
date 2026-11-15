# \ExecutionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_executions_list**](ExecutionsApi.md#function_executions_list) | **GET** /v1/_/function/executions | Query execution history logs across all endpoints



## function_executions_list

> models::ListExecutionsResponse function_executions_list(limit, offset, endpoint_id)
Query execution history logs across all endpoints

Queries execution telemetry logs from function.executions across all endpoints, returning status codes, execution duration in milliseconds, memory utilization, and stdout/stderr streams. Requires function:endpoint.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**limit** | Option<**i32**> | Maximum number of items to return (default 50, maximum 1000) |  |[default to 50]
**offset** | Option<**i32**> | Number of items to skip before returning results (default 0) |  |[default to 0]
**endpoint_id** | Option<**String**> | Filter executions by endpoint UUID |  |

### Return type

[**models::ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

