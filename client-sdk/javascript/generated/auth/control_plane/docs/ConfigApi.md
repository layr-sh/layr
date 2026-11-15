# ConfigApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authConfigGet**](ConfigApi.md#authconfigget) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration |
| [**authConfigUpdate**](ConfigApi.md#authconfigupdate) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration |



## authConfigGet

> Config authConfigGet()

Retrieve dynamic runtime authentication configuration

Returns active dynamic authentication settings from auth.config including password strength rules, session TTLs, rate limits, MFA policies, and enabled OAuth providers, with client secrets projected as boolean indicators. Requires auth:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { AuthConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.authConfigGet();
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

[**Config**](Config.md)

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


## authConfigUpdate

> Config authConfigUpdate(config)

Update dynamic runtime authentication configuration

Updates dynamic authentication configuration parameters in auth.config without requiring server restarts. Automatically envelope-encrypts newly supplied third-party OAuth provider secrets with AES-256-GCM using the master encryption key. Requires auth:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { AuthConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for auth.Config
    config: ...,
  } satisfies AuthConfigUpdateRequest;

  try {
    const data = await api.authConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for auth.Config | |

### Return type

[**Config**](Config.md)

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

