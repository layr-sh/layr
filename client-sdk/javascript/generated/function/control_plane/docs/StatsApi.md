# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionStatsGet**](StatsApi.md#functionstatsget) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics |



## functionStatsGet

> GetStatsResponse functionStatsGet()

Get function runtime telemetry and worker statistics

Returns runtime health, endpoint and deployment counts, stored bundle payload bytes, domain and TLS status counts, retained execution totals, and execution counts, failures, and average/P95 latency over a rolling 24-hour window. Includes per-endpoint window usage. Failures include HTTP 4xx/5xx responses or recorded execution errors. Metrics reflect retained logs, not lifetime traffic. Requires function:stats.read scope.

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

