# CacheAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataCacheFlush**](CacheAPI.md#datacacheflush) | **POST** /v1/_/data/cache/flush | Flush all global query and schema reflection caches
[**dataCacheInvalidate**](CacheAPI.md#datacacheinvalidate) | **POST** /v1/_/data/cache/invalidate | Invalidate specific table or catalog caches


# **dataCacheFlush**
```swift
    open class func dataCacheFlush(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> FlushCacheResponse
```

Flush all global query and schema reflection caches

Flushes in-memory and distributed table reflection metadata, query result caches, and composite version counters across all cluster nodes. Requires data:cache.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Flush all global query and schema reflection caches
do {
    let response = try await CacheAPI.dataCacheFlush(body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**FlushCacheResponse**](FlushCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataCacheInvalidate**
```swift
    open class func dataCacheInvalidate(invalidateCacheInput: InvalidateCacheInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> InvalidateCacheResponse
```

Invalidate specific table or catalog caches

Invalidates schema reflection and query caches for a specific schema, table name, or pattern, bumping composite table cache versions. Requires data:cache.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let invalidateCacheInput = InvalidateCacheInput(all: false, catalog: false, pattern: "pattern_example", schema: "schema_example", table: "table_example") // InvalidateCacheInput | Request body for data.InvalidateCacheInput

// Invalidate specific table or catalog caches
do {
    let response = try await CacheAPI.dataCacheInvalidate(invalidateCacheInput: invalidateCacheInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **invalidateCacheInput** | [**InvalidateCacheInput**](InvalidateCacheInput.md) | Request body for data.InvalidateCacheInput | 

### Return type

[**InvalidateCacheResponse**](InvalidateCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

