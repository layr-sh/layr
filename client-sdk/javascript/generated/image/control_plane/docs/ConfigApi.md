# ConfigApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**imageConfigGet**](ConfigApi.md#imageconfigget) | **GET** /v1/_/image/config | Get runtime image service configuration |
| [**imageConfigUpdate**](ConfigApi.md#imageconfigupdate) | **PUT** /v1/_/image/config | Update runtime image service configuration |



## imageConfigGet

> Config imageConfigGet()

Get runtime image service configuration

Returns operational image service settings including compression quality defaults, maximum resolution limits, allowed source domains, and signature enforcement policy. Requires image:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { ImageConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.imageConfigGet();
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


## imageConfigUpdate

> Config imageConfigUpdate(config)

Update runtime image service configuration

Modifies operational image service parameters in image.config, updating compression quality, resolution limits, and domain allowlists without restarting the service. Requires image:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { ImageConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for image.Config
    config: ...,
  } satisfies ImageConfigUpdateRequest;

  try {
    const data = await api.imageConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for image.Config | |

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

