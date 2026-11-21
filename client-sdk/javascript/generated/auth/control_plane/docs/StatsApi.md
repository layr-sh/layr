# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authStatsGet**](StatsApi.md#authstatsget) | **GET** /v1/_/auth/stats | Get application authentication statistics |



## authStatsGet

> StatsResponse authStatsGet()

Get application authentication statistics

Returns directory-wide application user, credential, and session counts from a consistent database snapshot. Active sessions exclude expired, idle, and locked-user sessions. Requires auth:stats.read scope.

### Example

```ts
import {
  Configuration,
  StatsApi,
} from '';
import type { AuthStatsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new StatsApi();

  try {
    const data = await api.authStatsGet();
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

[**StatsResponse**](StatsResponse.md)

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

