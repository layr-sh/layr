# EndpointDomainsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionEndpointDomainsCreate**](EndpointDomainsApi.md#functionendpointdomainscreate) | **POST** /v1/_/function/endpoints/{endpoint_id}/domains | Attach endpoint route to custom domain with path prefix |
| [**functionEndpointDomainsDelete**](EndpointDomainsApi.md#functionendpointdomainsdelete) | **DELETE** /v1/_/function/endpoints/{endpoint_id}/domains/{route_id} | Remove endpoint route mapping from custom domain |
| [**functionEndpointDomainsList**](EndpointDomainsApi.md#functionendpointdomainslist) | **GET** /v1/_/function/endpoints/{endpoint_id}/domains | List custom domain routes attached to an endpoint |



## functionEndpointDomainsCreate

> CustomDomainRoute functionEndpointDomainsCreate(endpointId, createCustomDomainRouteInput)

Attach endpoint route to custom domain with path prefix

Maps a custom domain hostname and URL path prefix to a specific serverless endpoint in function.custom_domain_routes, enabling custom domain routing. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  EndpointDomainsApi,
} from '';
import type { FunctionEndpointDomainsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointDomainsApi();

  const body = {
    // string
    endpointId: endpointId_example,
    // CreateCustomDomainRouteInput | Request body for function.CreateCustomDomainRouteInput
    createCustomDomainRouteInput: ...,
  } satisfies FunctionEndpointDomainsCreateRequest;

  try {
    const data = await api.functionEndpointDomainsCreate(body);
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
| **createCustomDomainRouteInput** | [CreateCustomDomainRouteInput](CreateCustomDomainRouteInput.md) | Request body for function.CreateCustomDomainRouteInput | |

### Return type

[**CustomDomainRoute**](CustomDomainRoute.md)

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


## functionEndpointDomainsDelete

> any functionEndpointDomainsDelete(endpointId, routeId)

Remove endpoint route mapping from custom domain

Removes a specific domain route mapping from function.custom_domain_routes, unbinding the custom path prefix from the endpoint. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  EndpointDomainsApi,
} from '';
import type { FunctionEndpointDomainsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointDomainsApi();

  const body = {
    // string
    endpointId: endpointId_example,
    // string
    routeId: routeId_example,
  } satisfies FunctionEndpointDomainsDeleteRequest;

  try {
    const data = await api.functionEndpointDomainsDelete(body);
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
| **routeId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Custom domain route deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## functionEndpointDomainsList

> ListCustomDomainRoutesResponse functionEndpointDomainsList(endpointId)

List custom domain routes attached to an endpoint

Returns all custom domain route mappings from function.custom_domain_routes configured for a specific serverless endpoint. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  EndpointDomainsApi,
} from '';
import type { FunctionEndpointDomainsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EndpointDomainsApi();

  const body = {
    // string
    endpointId: endpointId_example,
  } satisfies FunctionEndpointDomainsListRequest;

  try {
    const data = await api.functionEndpointDomainsList(body);
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

[**ListCustomDomainRoutesResponse**](ListCustomDomainRoutesResponse.md)

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

