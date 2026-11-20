# \StatsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**file_storage_stats_get**](StatsApi.md#file_storage_stats_get) | **GET** /v1/_/file-storage/stats | Get file storage usage statistics



## file_storage_stats_get

> models::StatsResponse file_storage_stats_get()
Get file storage usage statistics

Reports bucket counts, tracked object counts and logical bytes by backend, database chunk payload bytes, pending multipart usage, per-bucket usage and content type distribution. S3 usage includes only locally registered objects, not untracked upstream contents. Requires storage:bucket.read scope.

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

