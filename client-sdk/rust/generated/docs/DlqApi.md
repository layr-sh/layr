# \DlqApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasks_dlq_list**](DlqApi.md#tasks_dlq_list) | **GET** /v1/_/tasks/dlq | List dead-lettered failed task executions
[**tasks_dlq_purge**](DlqApi.md#tasks_dlq_purge) | **DELETE** /v1/_/tasks/dlq/{execution_id} | Permanently purge a failed task from the DLQ
[**tasks_dlq_retry**](DlqApi.md#tasks_dlq_retry) | **POST** /v1/_/tasks/dlq/{execution_id}/retry | Re-queue a failed task execution from the DLQ



## tasks_dlq_list

> models::ListDlqResponse tasks_dlq_list()
List dead-lettered failed task executions

Queries dead-lettered executions from tasks.executions that exhausted all configured retry attempts, detailing failure errors, stack traces, and attempt counts. Requires tasks:execution.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListDlqResponse**](ListDLQResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_dlq_purge

> serde_json::Value tasks_dlq_purge(execution_id)
Permanently purge a failed task from the DLQ

Permanently removes a failed execution record from tasks.executions / dead-letter queue. Requires tasks:execution.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**execution_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_dlq_retry

> models::RetryDlqResponse tasks_dlq_retry(execution_id, body)
Re-queue a failed task execution from the DLQ

Resets a dead-lettered execution in tasks.executions to 'pending' status for immediate worker retry pickup. Requires tasks:execution.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**execution_id** | **String** |  | [required] |
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::RetryDlqResponse**](RetryDLQResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

