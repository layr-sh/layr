# \CacheApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_cache_flush**](CacheApi.md#data_cache_flush) | **POST** /v1/_/data/cache/flush | Flush all global query and schema reflection caches
[**data_cache_invalidate**](CacheApi.md#data_cache_invalidate) | **POST** /v1/_/data/cache/invalidate | Invalidate specific table or catalog caches



## data_cache_flush

> models::FlushCacheResponse data_cache_flush(body)
Flush all global query and schema reflection caches

Flushes in-memory and distributed table reflection metadata, query result caches, and composite version counters across all cluster nodes. Requires data:cache.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::FlushCacheResponse**](FlushCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_cache_invalidate

> models::InvalidateCacheResponse data_cache_invalidate(invalidate_cache_input)
Invalidate specific table or catalog caches

Invalidates schema reflection and query caches for a specific schema, table name, or pattern, bumping composite table cache versions. Requires data:cache.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**invalidate_cache_input** | [**InvalidateCacheInput**](InvalidateCacheInput.md) | Request body for data.InvalidateCacheInput | [required] |

### Return type

[**models::InvalidateCacheResponse**](InvalidateCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

