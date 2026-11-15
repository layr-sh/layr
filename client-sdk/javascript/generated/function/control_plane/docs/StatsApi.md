# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionStatsGet**](StatsApi.md#functionstatsget) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics |



## functionStatsGet

> GetStatsResponse functionStatsGet()

Get function runtime telemetry and worker statistics

Returns real-time runtime statistics including active worker isolates count, memory consumption, runner queue depth, and throughput metrics. Requires function:stats.read scope.

### Example

```ts
import {
  Configuration,
  StatsApi,
} from '';
import type { FunctionStatsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new StatsApi();

  try {
    const data = await api.functionStatsGet();
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

