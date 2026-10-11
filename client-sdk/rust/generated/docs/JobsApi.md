# \JobsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasks_jobs_create**](JobsApi.md#tasks_jobs_create) | **POST** /v1/_/tasks/jobs | Register a new recurring cron job
[**tasks_jobs_delete**](JobsApi.md#tasks_jobs_delete) | **DELETE** /v1/_/tasks/jobs/{job_id} | Delete a cron job and cancel queued executions
[**tasks_jobs_get**](JobsApi.md#tasks_jobs_get) | **GET** /v1/_/tasks/jobs/{job_id} | Get cron job definition by ID
[**tasks_jobs_list**](JobsApi.md#tasks_jobs_list) | **GET** /v1/_/tasks/jobs | List all recurring cron jobs
[**tasks_jobs_update**](JobsApi.md#tasks_jobs_update) | **PATCH** /v1/_/tasks/jobs/{job_id} | Update an existing cron job definition



## tasks_jobs_create

> models::Job tasks_jobs_create(create_job_input)
Register a new recurring cron job

Creates a recurring cron job in tasks.jobs with 5-field cron expression, target execution driver (http webhook, serverless function, or SQL stored procedure), payload JSON, timeout guards, retry limits, and timezone. Calculates next run timestamp and emits tasks.job.created event. Requires tasks:job.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_job_input** | [**CreateJobInput**](CreateJobInput.md) | Request body for tasks.CreateJobInput | [required] |

### Return type

[**models::Job**](Job.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_jobs_delete

> serde_json::Value tasks_jobs_delete(job_id)
Delete a cron job and cancel queued executions

Deletes a cron job definition from tasks.jobs and cancels any associated pending executions in tasks.executions. Emits tasks.job.deleted event. Requires tasks:job.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**job_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_jobs_get

> models::GetJobResponse tasks_jobs_get(job_id)
Get cron job definition by ID

Retrieves a single cron job definition from tasks.jobs by ID, including cron expression, target payload, execution driver, next run countdown, and status. Requires tasks:job.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**job_id** | **String** |  | [required] |

### Return type

[**models::GetJobResponse**](GetJobResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_jobs_list

> models::ListJobsResponse tasks_jobs_list()
List all recurring cron jobs

Returns all registered recurring cron jobs from tasks.jobs, detailing cron schedule expressions, target drivers (http, function, sql), target payloads, timezone, next execution countdowns, and active status. Requires tasks:job.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListJobsResponse**](ListJobsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## tasks_jobs_update

> models::Job tasks_jobs_update(job_id, update_job_input)
Update an existing cron job definition

Modifies cron schedule expressions, timezone, target payloads, retry parameters, or active status in tasks.jobs. Recalculates next run timestamp and emits tasks.job.updated event. Requires tasks:job.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**job_id** | **String** |  | [required] |
**update_job_input** | [**UpdateJobInput**](UpdateJobInput.md) | Request body for tasks.UpdateJobInput | [required] |

### Return type

[**models::Job**](Job.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

