# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**imageStatsGet**](StatsApi.md#imagestatsget) | **GET** /v1/_/image/stats | Get image service telemetry statistics |



## imageStatsGet

> GetStatsResponse imageStatsGet()

Get image service telemetry statistics

Returns real-time operational statistics including image cache hit ratios, processing latency, memory usage, and transformation throughput. Requires image:stats.read scope.

### Example

```ts
import {
  Configuration,
  StatsApi,
} from '';
import type { ImageStatsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new StatsApi();

  try {
    const data = await api.imageStatsGet();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**GetStatsResponse**](GetStatsResponse.md)

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

