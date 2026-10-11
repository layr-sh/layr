# \ExecutionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_executions_list**](ExecutionsApi.md#function_executions_list) | **GET** /v1/_/function/executions | Query execution history logs across all endpoints
[**tasks_executions_create**](ExecutionsApi.md#tasks_executions_create) | **POST** /v1/_/tasks/executions | Trigger an immediate out-of-schedule execution
[**tasks_executions_list**](ExecutionsApi.md#tasks_executions_list) | **GET** /v1/_/tasks/executions | Query execution audit logs



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


## tasks_executions_create

> models::TriggerExecutionResponse tasks_executions_create(trigger_execution_input)
Trigger an immediate out-of-schedule execution

Manually enqueues an execution for a job in tasks.executions with status 'pending' for immediate worker dispatch out-of-schedule. Requires tasks:execution.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**trigger_execution_input** | [**TriggerExecutionInput**](TriggerExecutionInput.md) | Request body for tasks.TriggerExecutionInput | [required] |

### Return type

[**models::TriggerExecutionResponse**](TriggerExecutionResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_executions_list

> models::ListExecutionsResponse tasks_executions_list(limit, offset, job_id)
Query execution audit logs

Queries execution history records from tasks.executions with filtering by job ID, execution status (pending, running, completed, failed, dead_lettered), duration, and date ranges. Requires tasks:execution.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**limit** | Option<**i32**> | Maximum number of items to return (default 50, maximum 1000) |  |[default to 50]
**offset** | Option<**i32**> | Number of items to skip before returning results (default 0) |  |[default to 0]
**job_id** | Option<**String**> | Filter task executions by job UUID |  |

### Return type

[**models::ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

