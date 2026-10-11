# JobsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**tasksJobsCreate**](JobsApi.md#tasksjobscreate) | **POST** /v1/_/tasks/jobs | Register a new recurring cron job |
| [**tasksJobsDelete**](JobsApi.md#tasksjobsdelete) | **DELETE** /v1/_/tasks/jobs/{job_id} | Delete a cron job and cancel queued executions |
| [**tasksJobsGet**](JobsApi.md#tasksjobsget) | **GET** /v1/_/tasks/jobs/{job_id} | Get cron job definition by ID |
| [**tasksJobsList**](JobsApi.md#tasksjobslist) | **GET** /v1/_/tasks/jobs | List all recurring cron jobs |
| [**tasksJobsUpdate**](JobsApi.md#tasksjobsupdate) | **PATCH** /v1/_/tasks/jobs/{job_id} | Update an existing cron job definition |



## tasksJobsCreate

> Job tasksJobsCreate(createJobInput)

Register a new recurring cron job

Creates a recurring cron job in tasks.jobs with 5-field cron expression, target execution driver (http webhook, serverless function, or SQL stored procedure), payload JSON, timeout guards, retry limits, and timezone. Calculates next run timestamp and emits tasks.job.created event. Requires tasks:job.write scope.

### Example

```ts
import {
  Configuration,
  JobsApi,
} from '';
import type { TasksJobsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new JobsApi();

  const body = {
    // CreateJobInput | Request body for tasks.CreateJobInput
    createJobInput: ...,
  } satisfies TasksJobsCreateRequest;

  try {
    const data = await api.tasksJobsCreate(body);
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
| **createJobInput** | [CreateJobInput](CreateJobInput.md) | Request body for tasks.CreateJobInput | |

### Return type

[**Job**](Job.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## tasksJobsDelete

> any tasksJobsDelete(jobId)

Delete a cron job and cancel queued executions

Deletes a cron job definition from tasks.jobs and cancels any associated pending executions in tasks.executions. Emits tasks.job.deleted event. Requires tasks:job.write scope.

### Example

```ts
import {
  Configuration,
  JobsApi,
} from '';
import type { TasksJobsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new JobsApi();

  const body = {
    // string
    jobId: jobId_example,
  } satisfies TasksJobsDeleteRequest;

  try {
    const data = await api.tasksJobsDelete(body);
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
| **jobId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Job deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## tasksJobsGet

> GetJobResponse tasksJobsGet(jobId)

Get cron job definition by ID

Retrieves a single cron job definition from tasks.jobs by ID, including cron expression, target payload, execution driver, next run countdown, and status. Requires tasks:job.read scope.

### Example

```ts
import {
  Configuration,
  JobsApi,
} from '';
import type { TasksJobsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new JobsApi();

  const body = {
    // string
    jobId: jobId_example,
  } satisfies TasksJobsGetRequest;

  try {
    const data = await api.tasksJobsGet(body);
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
| **jobId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**GetJobResponse**](GetJobResponse.md)

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


## tasksJobsList

> ListJobsResponse tasksJobsList()

List all recurring cron jobs

Returns all registered recurring cron jobs from tasks.jobs, detailing cron schedule expressions, target drivers (http, function, sql), target payloads, timezone, next execution countdowns, and active status. Requires tasks:job.read scope.

### Example

```ts
import {
  Configuration,
  JobsApi,
} from '';
import type { TasksJobsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new JobsApi();

  try {
    const data = await api.tasksJobsList();
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

[**ListJobsResponse**](ListJobsResponse.md)

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


## tasksJobsUpdate

> Job tasksJobsUpdate(jobId, updateJobInput)

Update an existing cron job definition

Modifies cron schedule expressions, timezone, target payloads, retry parameters, or active status in tasks.jobs. Recalculates next run timestamp and emits tasks.job.updated event. Requires tasks:job.write scope.

### Example

```ts
import {
  Configuration,
  JobsApi,
} from '';
import type { TasksJobsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new JobsApi();

  const body = {
    // string
    jobId: jobId_example,
    // UpdateJobInput | Request body for tasks.UpdateJobInput
    updateJobInput: ...,
  } satisfies TasksJobsUpdateRequest;

  try {
    const data = await api.tasksJobsUpdate(body);
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
| **jobId** | `string` |  | [Defaults to `undefined`] |
| **updateJobInput** | [UpdateJobInput](UpdateJobInput.md) | Request body for tasks.UpdateJobInput | |

### Return type

[**Job**](Job.md)

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

