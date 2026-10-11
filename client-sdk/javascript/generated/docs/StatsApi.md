# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionStatsGet**](StatsApi.md#functionstatsget) | **GET** /v1/_/function/stats | Get function runtime telemetry and worker statistics |
| [**imageStatsGet**](StatsApi.md#imagestatsget) | **GET** /v1/_/image/stats | Get image service telemetry statistics |
| [**tasksStatsGet**](StatsApi.md#tasksstatsget) | **GET** /v1/_/tasks/stats | Get operational task telemetry and queue statistics |



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

