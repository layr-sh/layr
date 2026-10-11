# DLQApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**tasksDlqList**](DLQApi.md#tasksdlqlist) | **GET** /v1/_/tasks/dlq | List dead-lettered failed task executions |
| [**tasksDlqPurge**](DLQApi.md#tasksdlqpurge) | **DELETE** /v1/_/tasks/dlq/{execution_id} | Permanently purge a failed task from the DLQ |
| [**tasksDlqRetry**](DLQApi.md#tasksdlqretry) | **POST** /v1/_/tasks/dlq/{execution_id}/retry | Re-queue a failed task execution from the DLQ |



## tasksDlqList

> ListDLQResponse tasksDlqList()

List dead-lettered failed task executions

Queries dead-lettered executions from tasks.executions that exhausted all configured retry attempts, detailing failure errors, stack traces, and attempt counts. Requires tasks:execution.read scope.

### Example

```ts
import {
  Configuration,
  DLQApi,
} from '';
import type { TasksDlqListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DLQApi();

  try {
    const data = await api.tasksDlqList();
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

[**ListDLQResponse**](ListDLQResponse.md)

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


## tasksDlqPurge

> any tasksDlqPurge(executionId)

Permanently purge a failed task from the DLQ

Permanently removes a failed execution record from tasks.executions / dead-letter queue. Requires tasks:execution.write scope.

### Example

```ts
import {
  Configuration,
  DLQApi,
} from '';
import type { TasksDlqPurgeRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DLQApi();

  const body = {
    // string
    executionId: executionId_example,
  } satisfies TasksDlqPurgeRequest;

  try {
    const data = await api.tasksDlqPurge(body);
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
| **executionId** | `string` |  | [Defaults to `undefined`] |

### Return type

**any**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **204** | DLQ execution purged |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## tasksDlqRetry

> RetryDLQResponse tasksDlqRetry(executionId, body)

Re-queue a failed task execution from the DLQ

Resets a dead-lettered execution in tasks.executions to \&#39;pending\&#39; status for immediate worker retry pickup. Requires tasks:execution.write scope.

### Example

```ts
import {
  Configuration,
  DLQApi,
} from '';
import type { TasksDlqRetryRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DLQApi();

  const body = {
    // string
    executionId: executionId_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies TasksDlqRetryRequest;

  try {
    const data = await api.tasksDlqRetry(body);
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
| **executionId** | `string` |  | [Defaults to `undefined`] |
| **body** | `any` | Request body for core.Empty | |

### Return type

[**RetryDLQResponse**](RetryDLQResponse.md)

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

