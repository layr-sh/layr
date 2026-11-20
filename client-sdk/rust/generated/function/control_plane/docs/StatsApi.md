# \StatsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_stats_get**](StatsApi.md#function_stats_get) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics



## function_stats_get

> models::GetStatsResponse function_stats_get()
Get function runtime telemetry and worker statistics

Returns runtime health, endpoint and deployment counts, stored bundle payload bytes, domain and TLS status counts, retained execution totals, and execution counts, failures, and average/P95 latency over a rolling 24-hour window. Includes per-endpoint window usage. Failures include HTTP 4xx/5xx responses or recorded execution errors. Metrics reflect retained logs, not lifetime traffic. Requires function:stats.read scope.

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

