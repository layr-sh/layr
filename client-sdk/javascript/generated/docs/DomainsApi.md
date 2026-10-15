# DomainsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**functionDomainsCreate**](DomainsApi.md#functiondomainscreate) | **POST** /v1/_/function/domains | Register a standalone custom domain |
| [**functionDomainsDelete**](DomainsApi.md#functiondomainsdelete) | **DELETE** /v1/_/function/domains/{domain_id} | Delete custom domain and cascade attached routes |
| [**functionDomainsGet**](DomainsApi.md#functiondomainsget) | **GET** /v1/_/function/domains/{domain_id} | Get custom domain details and DNS instructions |
| [**functionDomainsList**](DomainsApi.md#functiondomainslist) | **GET** /v1/_/function/domains | List registered custom domains |
| [**functionDomainsUpdate**](DomainsApi.md#functiondomainsupdate) | **PATCH** /v1/_/function/domains/{domain_id} | Update custom domain configuration |
| [**functionDomainsVerify**](DomainsApi.md#functiondomainsverify) | **POST** /v1/_/function/domains/{domain_id}/verify | Verify custom domain DNS configuration |



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


## functionDomainsGet

> CustomDomain functionDomainsGet(domainId)

Get custom domain details and DNS instructions

Returns a single custom domain definition by UUID from function.custom_domains including verification status, TLS configuration, and DNS setup instructions. Requires function:endpoint.read scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  const body = {
    // string
    domainId: domainId_example,
  } satisfies FunctionDomainsGetRequest;

  try {
    const data = await api.functionDomainsGet(body);
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

[**CustomDomain**](CustomDomain.md)

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


## functionDomainsUpdate

> CustomDomain functionDomainsUpdate(domainId, updateCustomDomainInput)

Update custom domain configuration

Updates custom domain configuration such as toggling TLS on or off, or updating ACME contact email at runtime. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  const body = {
    // string
    domainId: domainId_example,
    // UpdateCustomDomainInput | Request body for function.UpdateCustomDomainInput
    updateCustomDomainInput: ...,
  } satisfies FunctionDomainsUpdateRequest;

  try {
    const data = await api.functionDomainsUpdate(body);
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
| **updateCustomDomainInput** | [UpdateCustomDomainInput](UpdateCustomDomainInput.md) | Request body for function.UpdateCustomDomainInput | |

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
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## functionDomainsVerify

> CustomDomain functionDomainsVerify(domainId, body)

Verify custom domain DNS configuration

Performs a live DNS lookup (CNAME/IP) to verify domain DNS points to Layr ingress. If matched, activates domain status and triggers TLS provisioning when enabled. If not matched, returns misconfigured status with diagnostic error details. Requires function:endpoint.write scope.

### Example

```ts
import {
  Configuration,
  DomainsApi,
} from '';
import type { FunctionDomainsVerifyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new DomainsApi();

  const body = {
    // string
    domainId: domainId_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies FunctionDomainsVerifyRequest;

  try {
    const data = await api.functionDomainsVerify(body);
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
| **body** | `any` | Request body for core.Empty | |

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
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

