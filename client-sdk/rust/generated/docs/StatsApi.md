# \StatsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_stats_get**](StatsApi.md#function_stats_get) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics
[**image_stats_get**](StatsApi.md#image_stats_get) | **GET** /v1/_/image/stats | Get image service telemetry statistics
[**tasks_stats_get**](StatsApi.md#tasks_stats_get) | **GET** /v1/_/tasks/stats | Get operational task telemetry and queue statistics



## function_stats_get

> models::GetStatsResponse function_stats_get()
Get function runtime telemetry and worker statistics

Returns real-time runtime statistics including active worker isolates count, memory consumption, runner queue depth, and throughput metrics. Requires function:stats.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_stats_get

> models::GetStatsResponse image_stats_get()
Get image service telemetry statistics

Returns real-time operational statistics including image cache hit ratios, processing latency, memory usage, and transformation throughput. Requires image:stats.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_stats_get

> models::GetStatsResponse tasks_stats_get()
Get operational task telemetry and queue statistics

Returns real-time task statistics including total job counts, active queue depth, currently running executions, DLQ size, and execution success rate. Requires tasks:stats.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetStatsResponse**](GetStatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

