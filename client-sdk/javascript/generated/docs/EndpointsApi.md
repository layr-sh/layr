# EndpointsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionEndpointsCreate**](EndpointsApi.md#functionendpointscreate) | **POST** /v1/_/function/endpoints | Register a new serverless endpoint |
| [**functionEndpointsDelete**](EndpointsApi.md#functionendpointsdelete) | **DELETE** /v1/_/function/endpoints/{endpoint_id} | Delete serverless endpoint and unload runtime isolates |
| [**functionEndpointsExecutions**](EndpointsApi.md#functionendpointsexecutions) | **GET** /v1/_/function/endpoints/{endpoint_id}/executions | Query execution history logs for a specific endpoint |
| [**functionEndpointsGet**](EndpointsApi.md#functionendpointsget) | **GET** /v1/_/function/endpoints/{endpoint_id} | Get serverless endpoint definition by ID |
| [**functionEndpointsList**](EndpointsApi.md#functionendpointslist) | **GET** /v1/_/function/endpoints | List all registered serverless endpoints |
| [**functionEndpointsUpdate**](EndpointsApi.md#functionendpointsupdate) | **PUT** /v1/_/function/endpoints/{endpoint_id} | Update serverless endpoint definition |



## functionEndpointsCreate

> Endpoint functionEndpointsCreate(createEndpointInput)

Register a new serverless endpoint

Provisions a new serverless endpoint definition in function.endpoints with route path, name, runtime environment (Node, Deno, Python, Go), timeout guards, memory limits, and environment variables. Emits function.endpoint.created event. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  const body = {
    // CreateEndpointInput | Request body for function.CreateEndpointInput
    createEndpointInput: ...,
  } satisfies FunctionEndpointsCreateRequest;

  try {
    const data = await api.functionEndpointsCreate(body);
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
| **createEndpointInput** | [CreateEndpointInput](CreateEndpointInput.md) | Request body for function.CreateEndpointInput | |

### Return type

[**Endpoint**](Endpoint.md)

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


## functionEndpointsDelete

> any functionEndpointsDelete(endpointId)

Delete serverless endpoint and unload runtime isolates

Permanently deletes an endpoint from function.endpoints, unloads running worker isolates, cascades historical deployments, and removes custom domain routes. Emits function.endpoint.deleted event. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  const body = {
    // string
    endpointId: endpointId_example,
  } satisfies FunctionEndpointsDeleteRequest;

  try {
    const data = await api.functionEndpointsDelete(body);
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
| **endpointId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Endpoint deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## functionEndpointsExecutions

> ListExecutionsResponse functionEndpointsExecutions(endpointId)

Query execution history logs for a specific endpoint

Returns historical invocation audit records from function.executions for a specific endpoint, detailing response codes, durations, memory usage, and execution log streams. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsExecutionsRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  const body = {
    // string
    endpointId: endpointId_example,
  } satisfies FunctionEndpointsExecutionsRequest;

  try {
    const data = await api.functionEndpointsExecutions(body);
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
| **endpointId** | `string` |  | [Defaults to `undefined`] |

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


## functionEndpointsGet

> Endpoint functionEndpointsGet(endpointId)

Get serverless endpoint definition by ID

Retrieves complete definition of a serverless endpoint from function.endpoints by ID or name, including runtime configuration, environment variables, and active deployment version. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  const body = {
    // string
    endpointId: endpointId_example,
  } satisfies FunctionEndpointsGetRequest;

  try {
    const data = await api.functionEndpointsGet(body);
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
| **endpointId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Endpoint**](Endpoint.md)

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


## functionEndpointsList

> ListEndpointsResponse functionEndpointsList()

List all registered serverless endpoints

Returns all registered serverless endpoints from function.endpoints with route names, runtime engines, timeout guards, memory limits, active deployment IDs, and status. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  try {
    const data = await api.functionEndpointsList();
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

[**ListEndpointsResponse**](ListEndpointsResponse.md)

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


## functionEndpointsUpdate

> Endpoint functionEndpointsUpdate(endpointId, updateEndpointInput)

Update serverless endpoint definition

Modifies an existing serverless endpoint definition in function.endpoints, updating environment variables, timeout guards, or memory limits. Emits function.endpoint.updated event. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  EndpointsApi,
} from '';
import type { FunctionEndpointsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointsApi();

  const body = {
    // string
    endpointId: endpointId_example,
    // UpdateEndpointInput | Request body for function.UpdateEndpointInput
    updateEndpointInput: ...,
  } satisfies FunctionEndpointsUpdateRequest;

  try {
    const data = await api.functionEndpointsUpdate(body);
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
| **endpointId** | `string` |  | [Defaults to `undefined`] |
| **updateEndpointInput** | [UpdateEndpointInput](UpdateEndpointInput.md) | Request body for function.UpdateEndpointInput | |

### Return type

[**Endpoint**](Endpoint.md)

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

