# DeploymentsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionDeploymentsCreate**](DeploymentsApi.md#functiondeploymentscreate) | **POST** /v1/_/function/endpoints/{endpoint_id}/deploy | Create immutable deployment and hot-reload runner |
| [**functionDeploymentsList**](DeploymentsApi.md#functiondeploymentslist) | **GET** /v1/_/function/endpoints/{endpoint_id}/deployments | List immutable deployments for an endpoint |
| [**functionDeploymentsRollback**](DeploymentsApi.md#functiondeploymentsrollback) | **POST** /v1/_/function/endpoints/{endpoint_id}/rollback | Rollback endpoint to a previous deployment revision |



## functionDeploymentsCreate

> Deployment functionDeploymentsCreate(endpointId, createDeploymentInput)

Create immutable deployment and hot-reload runner

Uploads and registers an immutable code bundle in function.deployments, increments revision version, sets it as the active deployment pointer, and immediately hot-reloads runtime worker isolates. Emits function.deployment.created event. Requires function:endpoint.deploy scope.

### Example

```ts
import {
  Configuration,
  DeploymentsApi,
} from '';
import type { FunctionDeploymentsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DeploymentsApi();

  const body = {
    // string
    endpointId: endpointId_example,
    // CreateDeploymentInput | Request body for function.CreateDeploymentInput
    createDeploymentInput: ...,
  } satisfies FunctionDeploymentsCreateRequest;

  try {
    const data = await api.functionDeploymentsCreate(body);
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
| **createDeploymentInput** | [CreateDeploymentInput](CreateDeploymentInput.md) | Request body for function.CreateDeploymentInput | |

### Return type

[**Deployment**](Deployment.md)

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


## functionDeploymentsList

> ListDeploymentsResponse functionDeploymentsList(endpointId)

List immutable deployments for an endpoint

Returns version history of immutable code deployments for an endpoint from function.deployments, detailing revision numbers, creation timestamps, and active flags. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  DeploymentsApi,
} from '';
import type { FunctionDeploymentsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DeploymentsApi();

  const body = {
    // string
    endpointId: endpointId_example,
  } satisfies FunctionDeploymentsListRequest;

  try {
    const data = await api.functionDeploymentsList(body);
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

[**ListDeploymentsResponse**](ListDeploymentsResponse.md)

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


## functionDeploymentsRollback

> Deployment functionDeploymentsRollback(endpointId, rollbackDeploymentInput)

Rollback endpoint to a previous deployment revision

Atomically updates the active deployment pointer in function.endpoints to an earlier immutable deployment version, instantly hot-reloading runtime runner isolates. Emits function.deployment.rollback event. Requires function:endpoint.deploy scope.

### Example

```ts
import {
  Configuration,
  DeploymentsApi,
} from '';
import type { FunctionDeploymentsRollbackRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DeploymentsApi();

  const body = {
    // string
    endpointId: endpointId_example,
    // RollbackDeploymentInput | Request body for function.RollbackDeploymentInput
    rollbackDeploymentInput: ...,
  } satisfies FunctionDeploymentsRollbackRequest;

  try {
    const data = await api.functionDeploymentsRollback(body);
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
| **rollbackDeploymentInput** | [RollbackDeploymentInput](RollbackDeploymentInput.md) | Request body for function.RollbackDeploymentInput | |

### Return type

[**Deployment**](Deployment.md)

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

