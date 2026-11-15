# ConfigApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataConfigGet**](ConfigApi.md#dataconfigget) | **GET** /v1/_/data/config | Retrieve dynamic data service configuration |
| [**dataConfigUpdate**](ConfigApi.md#dataconfigupdate) | **PUT** /v1/_/data/config | Update dynamic data service configuration |



## dataConfigGet

> Config dataConfigGet()

Retrieve dynamic data service configuration

Returns dynamic data service configuration settings including exposed schema names, connection pool sizes, REST limit defaults, and cache TTL parameters. Requires data:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { DataConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.dataConfigGet();
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


## dataConfigUpdate

> Config dataConfigUpdate(config)

Update dynamic data service configuration

Updates operational data service parameters in data.config, dynamically tuning connection limits, exposed schemas, and cache behaviors without server downtime. Requires data:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { DataConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for data.Config
    config: ...,
  } satisfies DataConfigUpdateRequest;

  try {
    const data = await api.dataConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for data.Config | |

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

