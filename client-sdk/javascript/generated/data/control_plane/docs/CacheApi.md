# CacheApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataCacheFlush**](CacheApi.md#datacacheflush) | **POST** /v1/_/data/cache/flush | Flush all global query and schema reflection caches |
| [**dataCacheInvalidate**](CacheApi.md#datacacheinvalidate) | **POST** /v1/_/data/cache/invalidate | Invalidate specific table or catalog caches |



## dataCacheFlush

> FlushCacheResponse dataCacheFlush(body)

Flush all global query and schema reflection caches

Flushes in-memory and distributed table reflection metadata, query result caches, and composite version counters across all cluster nodes. Requires data:cache.write scope.

### Example

```ts
import {
  Configuration,
  CacheApi,
} from '';
import type { DataCacheFlushRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new CacheApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies DataCacheFlushRequest;

  try {
    const data = await api.dataCacheFlush(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **body** | `any` | Request body for core.Empty | |

### Return type

[**FlushCacheResponse**](FlushCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataCacheInvalidate

> InvalidateCacheResponse dataCacheInvalidate(invalidateCacheInput)

Invalidate specific table or catalog caches

Invalidates schema reflection and query caches for a specific schema, table name, or pattern, bumping composite table cache versions. Requires data:cache.write scope.

### Example

```ts
import {
  Configuration,
  CacheApi,
} from '';
import type { DataCacheInvalidateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new CacheApi();

  const body = {
    // InvalidateCacheInput | Request body for data.InvalidateCacheInput
    invalidateCacheInput: ...,
  } satisfies DataCacheInvalidateRequest;

  try {
    const data = await api.dataCacheInvalidate(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **invalidateCacheInput** | [InvalidateCacheInput](InvalidateCacheInput.md) | Request body for data.InvalidateCacheInput | |

### Return type

[**InvalidateCacheResponse**](InvalidateCacheResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

