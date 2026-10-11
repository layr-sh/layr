# DomainsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionDomainsCreate**](DomainsApi.md#functiondomainscreate) | **POST** /v1/_/function/domains | Register a standalone custom domain |
| [**functionDomainsDelete**](DomainsApi.md#functiondomainsdelete) | **DELETE** /v1/_/function/domains/{domain_id} | Delete custom domain and cascade attached routes |
| [**functionDomainsList**](DomainsApi.md#functiondomainslist) | **GET** /v1/_/function/domains | List registered custom domains |



## functionDomainsCreate

> CustomDomain functionDomainsCreate(createCustomDomainInput)

Register a standalone custom domain

Registers a standalone custom domain hostname in function.custom_domains with verification status and TLS configuration. Emits function.domain.created event. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  const body = {
    // CreateCustomDomainInput | Request body for function.CreateCustomDomainInput
    createCustomDomainInput: ...,
  } satisfies FunctionDomainsCreateRequest;

  try {
    const data = await api.functionDomainsCreate(body);
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
| **createCustomDomainInput** | [CreateCustomDomainInput](CreateCustomDomainInput.md) | Request body for function.CreateCustomDomainInput | |

### Return type

[**CustomDomain**](CustomDomain.md)

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


## functionDomainsDelete

> any functionDomainsDelete(domainId)

Delete custom domain and cascade attached routes

Deletes a custom domain from function.custom_domains, cascading removal of all attached endpoint route mappings. Emits function.domain.deleted event. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  const body = {
    // string
    domainId: domainId_example,
  } satisfies FunctionDomainsDeleteRequest;

  try {
    const data = await api.functionDomainsDelete(body);
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
| **domainId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Custom domain deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## functionDomainsList

> ListCustomDomainsResponse functionDomainsList()

List registered custom domains

Returns all registered custom domain definitions from function.custom_domains with hostnames and verification states. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  try {
    const data = await api.functionDomainsList();
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

[**ListCustomDomainsResponse**](ListCustomDomainsResponse.md)

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

