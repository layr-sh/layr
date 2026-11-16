# ConfigApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreConfigGet**](ConfigApi.md#coreconfigget) | **GET** /v1/_/core/config | Get Core runtime configuration |
| [**coreConfigUpdate**](ConfigApi.md#coreconfigupdate) | **PUT** /v1/_/core/config | Update Core runtime configuration |



## coreConfigGet

> RuntimeConfig coreConfigGet()

Get Core runtime configuration

#### Controller:   &#x60;layr.sh/core.(*Server).handleGetConfig&#x60;  ---  

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { CoreConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.coreConfigGet();
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

[**RuntimeConfig**](RuntimeConfig.md)

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


## coreConfigUpdate

> RuntimeConfig coreConfigUpdate(runtimeConfig)

Update Core runtime configuration

#### Controller:   &#x60;layr.sh/core.(*Server).handleUpdateConfig&#x60;  ---  

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { CoreConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // RuntimeConfig | Request body for core.RuntimeConfig
    runtimeConfig: ...,
  } satisfies CoreConfigUpdateRequest;

  try {
    const data = await api.coreConfigUpdate(body);
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
| **runtimeConfig** | [RuntimeConfig](RuntimeConfig.md) | Request body for core.RuntimeConfig | |

### Return type

[**RuntimeConfig**](RuntimeConfig.md)

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

