# ExecutionsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionExecutionsList**](ExecutionsApi.md#functionexecutionslist) | **GET** /v1/_/function/executions | Query execution history logs across all endpoints |



## functionExecutionsList

> ListExecutionsResponse functionExecutionsList(limit, offset, endpointId)

Query execution history logs across all endpoints

Queries execution telemetry logs from function.executions across all endpoints, returning status codes, execution duration in milliseconds, memory utilization, and stdout/stderr streams. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  ExecutionsApi,
} from '';
import type { FunctionExecutionsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ExecutionsApi();

  const body = {
    // number | Maximum number of items to return (default 50, maximum 1000) (optional)
    limit: 56,
    // number | Number of items to skip before returning results (default 0) (optional)
    offset: 56,
    // string | Filter executions by endpoint UUID (optional)
    endpointId: endpointId_example,
  } satisfies FunctionExecutionsListRequest;

  try {
    const data = await api.functionExecutionsList(body);
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
| **endpointId** | `string` | Filter executions by endpoint UUID | [Optional] [Defaults to `undefined`] |

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

