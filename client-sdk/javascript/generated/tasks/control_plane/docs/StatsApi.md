# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**tasksStatsGet**](StatsApi.md#tasksstatsget) | **GET** /v1/_/tasks/stats | Get operational task telemetry and queue statistics |



## tasksStatsGet

> GetStatsResponse tasksStatsGet()

Get operational task telemetry and queue statistics

Returns real-time task statistics including total job counts, active queue depth, currently running executions, DLQ size, and execution success rate. Requires tasks:stats.read scope.

### Example

```ts
import {
  Configuration,
  StatsApi,
} from '';
import type { TasksStatsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new StatsApi();

  try {
    const data = await api.tasksStatsGet();
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

