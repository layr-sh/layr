# JobsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasksJobsCreate**](JobsAPI.md#tasksjobscreate) | **POST** /v1/_/tasks/jobs | Register a new recurring cron job
[**tasksJobsDelete**](JobsAPI.md#tasksjobsdelete) | **DELETE** /v1/_/tasks/jobs/{job_id} | Delete a cron job and cancel queued executions
[**tasksJobsGet**](JobsAPI.md#tasksjobsget) | **GET** /v1/_/tasks/jobs/{job_id} | Get cron job definition by ID
[**tasksJobsList**](JobsAPI.md#tasksjobslist) | **GET** /v1/_/tasks/jobs | List all recurring cron jobs
[**tasksJobsUpdate**](JobsAPI.md#tasksjobsupdate) | **PATCH** /v1/_/tasks/jobs/{job_id} | Update an existing cron job definition


# **tasksJobsCreate**
```swift
    open class func tasksJobsCreate(createJobInput: CreateJobInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Job
```

Register a new recurring cron job

Creates a recurring cron job in tasks.jobs with 5-field cron expression, target execution driver (http webhook, serverless function, or SQL stored procedure), payload JSON, timeout guards, retry limits, and timezone. Calculates next run timestamp and emits tasks.job.created event. Requires tasks:job.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createJobInput = CreateJobInput(cronExpression: "cronExpression_example", isEnabled: false, name: "name_example", targetPayload: 123, targetType: "targetType_example", timezone: "timezone_example") // CreateJobInput | Request body for tasks.CreateJobInput

// Register a new recurring cron job
do {
    let response = try await JobsAPI.tasksJobsCreate(createJobInput: createJobInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createJobInput** | [**CreateJobInput**](CreateJobInput.md) | Request body for tasks.CreateJobInput | 

### Return type

[**Job**](Job.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksJobsDelete**
```swift
    open class func tasksJobsDelete(jobId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete a cron job and cancel queued executions

Deletes a cron job definition from tasks.jobs and cancels any associated pending executions in tasks.executions. Emits tasks.job.deleted event. Requires tasks:job.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let jobId = "jobId_example" // String | 

// Delete a cron job and cancel queued executions
do {
    let response = try await JobsAPI.tasksJobsDelete(jobId: jobId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **jobId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksJobsGet**
```swift
    open class func tasksJobsGet(jobId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetJobResponse
```

Get cron job definition by ID

Retrieves a single cron job definition from tasks.jobs by ID, including cron expression, target payload, execution driver, next run countdown, and status. Requires tasks:job.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let jobId = "jobId_example" // String | 

// Get cron job definition by ID
do {
    let response = try await JobsAPI.tasksJobsGet(jobId: jobId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **jobId** | **String** |  | 

### Return type

[**GetJobResponse**](GetJobResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksJobsList**
```swift
    open class func tasksJobsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListJobsResponse
```

List all recurring cron jobs

Returns all registered recurring cron jobs from tasks.jobs, detailing cron schedule expressions, target drivers (http, function, sql), target payloads, timezone, next execution countdowns, and active status. Requires tasks:job.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List all recurring cron jobs
do {
    let response = try await JobsAPI.tasksJobsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListJobsResponse**](ListJobsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksJobsUpdate**
```swift
    open class func tasksJobsUpdate(jobId: String, updateJobInput: UpdateJobInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Job
```

Update an existing cron job definition

Modifies cron schedule expressions, timezone, target payloads, retry parameters, or active status in tasks.jobs. Recalculates next run timestamp and emits tasks.job.updated event. Requires tasks:job.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let jobId = "jobId_example" // String | 
let updateJobInput = UpdateJobInput(cronExpression: "cronExpression_example", isEnabled: false, name: "name_example", targetPayload: 123, targetType: "targetType_example", timezone: "timezone_example") // UpdateJobInput | Request body for tasks.UpdateJobInput

// Update an existing cron job definition
do {
    let response = try await JobsAPI.tasksJobsUpdate(jobId: jobId, updateJobInput: updateJobInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **jobId** | **String** |  | 
 **updateJobInput** | [**UpdateJobInput**](UpdateJobInput.md) | Request body for tasks.UpdateJobInput | 

### Return type

[**Job**](Job.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

