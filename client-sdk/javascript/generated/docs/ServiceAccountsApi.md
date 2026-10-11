# ServiceAccountsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreServiceAccountsCreate**](ServiceAccountsApi.md#coreserviceaccountscreate) | **POST** /v1/_/core/service-accounts | Create a machine service account |
| [**coreServiceAccountsDelete**](ServiceAccountsApi.md#coreserviceaccountsdelete) | **DELETE** /v1/_/core/service-accounts/{service_account_id} | Delete machine service account |
| [**coreServiceAccountsGet**](ServiceAccountsApi.md#coreserviceaccountsget) | **GET** /v1/_/core/service-accounts/{service_account_id} | Get machine service account details by UUID |
| [**coreServiceAccountsList**](ServiceAccountsApi.md#coreserviceaccountslist) | **GET** /v1/_/core/service-accounts | List machine service accounts |
| [**coreServiceAccountsUpdate**](ServiceAccountsApi.md#coreserviceaccountsupdate) | **PUT** /v1/_/core/service-accounts/{service_account_id} | Update machine service account properties and scopes |



## coreServiceAccountsCreate

> CreateServiceAccountResponse coreServiceAccountsCreate(createServiceAccountInput)

Create a machine service account

Creates a machine service account, generates a high-entropy 32-byte hex secret key, hashes it with SHA-256 for storage in core.service_accounts, and returns the plaintext secret key once. Supports scope constraints and CIDR IP restrictions. Requires core:service-account.write scope.

### Example

```ts
import {
  Configuration,
  ServiceAccountsApi,
} from '';
import type { CoreServiceAccountsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ServiceAccountsApi();

  const body = {
    // CreateServiceAccountInput | Request body for core.CreateServiceAccountInput
    createServiceAccountInput: ...,
  } satisfies CoreServiceAccountsCreateRequest;

  try {
    const data = await api.coreServiceAccountsCreate(body);
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
| **createServiceAccountInput** | [CreateServiceAccountInput](CreateServiceAccountInput.md) | Request body for core.CreateServiceAccountInput | |

### Return type

[**CreateServiceAccountResponse**](CreateServiceAccountResponse.md)

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


## coreServiceAccountsDelete

> any coreServiceAccountsDelete(serviceAccountId)

Delete machine service account

Deletes a machine service account from core.service_accounts. Enforces strict safety invariants: prevents deleting accounts owned by console users and ensures at least one active root account remains. Requires core:service-account.write scope.

### Example

```ts
import {
  Configuration,
  ServiceAccountsApi,
} from '';
import type { CoreServiceAccountsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ServiceAccountsApi();

  const body = {
    // string
    serviceAccountId: serviceAccountId_example,
  } satisfies CoreServiceAccountsDeleteRequest;

  try {
    const data = await api.coreServiceAccountsDelete(body);
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
| **serviceAccountId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Service account deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## coreServiceAccountsGet

> ServiceAccount coreServiceAccountsGet(serviceAccountId)

Get machine service account details by UUID

Retrieves a machine service account by UUID from core.service_accounts, returning metadata, permission scopes, key prefix, and allowed IP patterns. Plaintext key is not recoverable. Requires core:service-account.read scope.

### Example

```ts
import {
  Configuration,
  ServiceAccountsApi,
} from '';
import type { CoreServiceAccountsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ServiceAccountsApi();

  const body = {
    // string
    serviceAccountId: serviceAccountId_example,
  } satisfies CoreServiceAccountsGetRequest;

  try {
    const data = await api.coreServiceAccountsGet(body);
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
| **serviceAccountId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**ServiceAccount**](ServiceAccount.md)

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


## coreServiceAccountsList

> Array&lt;ServiceAccount&gt; coreServiceAccountsList()

List machine service accounts

Lists all machine service accounts from core.service_accounts with name, key prefix, granted permission scopes, enabled status, IP allowlists, and expiration timestamps. Requires core:service-account.read scope.

### Example

```ts
import {
  Configuration,
  ServiceAccountsApi,
} from '';
import type { CoreServiceAccountsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ServiceAccountsApi();

  try {
    const data = await api.coreServiceAccountsList();
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

[**Array&lt;ServiceAccount&gt;**](ServiceAccount.md)

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


## coreServiceAccountsUpdate

> ServiceAccount coreServiceAccountsUpdate(serviceAccountId, updateServiceAccountInput)

Update machine service account properties and scopes

Modifies name, description, permission scopes, IP allowlist, or enabled status in core.service_accounts. Enforces safety invariant that at least one enabled root account with wildcard * scope remains. Requires core:service-account.write scope.

### Example

```ts
import {
  Configuration,
  ServiceAccountsApi,
} from '';
import type { CoreServiceAccountsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ServiceAccountsApi();

  const body = {
    // string
    serviceAccountId: serviceAccountId_example,
    // UpdateServiceAccountInput | Request body for core.UpdateServiceAccountInput
    updateServiceAccountInput: ...,
  } satisfies CoreServiceAccountsUpdateRequest;

  try {
    const data = await api.coreServiceAccountsUpdate(body);
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
| **serviceAccountId** | `string` |  | [Defaults to `undefined`] |
| **updateServiceAccountInput** | [UpdateServiceAccountInput](UpdateServiceAccountInput.md) | Request body for core.UpdateServiceAccountInput | |

### Return type

[**ServiceAccount**](ServiceAccount.md)

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

