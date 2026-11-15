# ExecutionsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**tasksExecutionsCreate**](ExecutionsApi.md#tasksexecutionscreate) | **POST** /v1/_/tasks/executions | Trigger an immediate out-of-schedule execution |
| [**tasksExecutionsList**](ExecutionsApi.md#tasksexecutionslist) | **GET** /v1/_/tasks/executions | Query execution audit logs |



## tasksExecutionsCreate

> TriggerExecutionResponse tasksExecutionsCreate(triggerExecutionInput)

Trigger an immediate out-of-schedule execution

Manually enqueues an execution for a job in tasks.executions with status \&#39;pending\&#39; for immediate worker dispatch out-of-schedule. Requires tasks:execution.write scope.

### Example

```ts
import {
  Configuration,
  ExecutionsApi,
} from '';
import type { TasksExecutionsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ExecutionsApi();

  const body = {
    // TriggerExecutionInput | Request body for tasks.TriggerExecutionInput
    triggerExecutionInput: ...,
  } satisfies TasksExecutionsCreateRequest;

  try {
    const data = await api.tasksExecutionsCreate(body);
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
| **triggerExecutionInput** | [TriggerExecutionInput](TriggerExecutionInput.md) | Request body for tasks.TriggerExecutionInput | |

### Return type

[**TriggerExecutionResponse**](TriggerExecutionResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **202** | Accepted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## tasksExecutionsList

> ListExecutionsResponse tasksExecutionsList(limit, offset, jobId)

Query execution audit logs

Queries execution history records from tasks.executions with filtering by job ID, execution status (pending, running, completed, failed, dead_lettered), duration, and date ranges. Requires tasks:execution.read scope.

### Example

```ts
import {
  Configuration,
  ExecutionsApi,
} from '';
import type { TasksExecutionsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ExecutionsApi();

  const body = {
    // number | Maximum number of items to return (default 50, maximum 1000) (optional)
    limit: 56,
    // number | Number of items to skip before returning results (default 0) (optional)
    offset: 56,
    // string | Filter task executions by job UUID (optional)
    jobId: jobId_example,
  } satisfies TasksExecutionsListRequest;

  try {
    const data = await api.tasksExecutionsList(body);
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
| **limit** | `number` | Maximum number of items to return (default 50, maximum 1000) | [Optional] [Defaults to `50`] |
| **offset** | `number` | Number of items to skip before returning results (default 0) | [Optional] [Defaults to `0`] |
| **jobId** | `string` | Filter task executions by job UUID | [Optional] [Defaults to `undefined`] |

### Return type

[**ListExecutionsResponse**](ListExecutionsResponse.md)

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

