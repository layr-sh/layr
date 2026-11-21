# \StatsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_stats_get**](StatsApi.md#auth_stats_get) | **GET** /v1/_/auth/stats | Get application authentication statistics



## auth_stats_get

> models::StatsResponse auth_stats_get()
Get application authentication statistics

Returns directory-wide application user, credential, and session counts from a consistent database snapshot. Active sessions exclude expired, idle, and locked-user sessions. Requires auth:stats.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::StatsResponse**](StatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

