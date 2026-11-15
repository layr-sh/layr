# \StatsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasks_stats_get**](StatsApi.md#tasks_stats_get) | **GET** /v1/_/tasks/stats | Get operational task telemetry and queue statistics



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

