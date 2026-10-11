# ConfigApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authConfigGet**](ConfigApi.md#authconfigget) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration |
| [**authConfigUpdate**](ConfigApi.md#authconfigupdate) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration |
| [**consoleConfigGet**](ConfigApi.md#consoleconfigget) | **GET** /v1/_/console/config | Get dynamic runtime console configuration |
| [**consoleConfigUpdate**](ConfigApi.md#consoleconfigupdate) | **PUT** /v1/_/console/config | Update dynamic runtime console configuration |
| [**dataConfigGet**](ConfigApi.md#dataconfigget) | **GET** /v1/_/data/config | Retrieve dynamic data service configuration |
| [**dataConfigUpdate**](ConfigApi.md#dataconfigupdate) | **PUT** /v1/_/data/config | Update dynamic data service configuration |
| [**fileStorageConfigGet**](ConfigApi.md#filestorageconfigget) | **GET** /v1/_/file-storage/config | Get runtime file storage configuration |
| [**fileStorageConfigUpdate**](ConfigApi.md#filestorageconfigupdate) | **PUT** /v1/_/file-storage/config | Update runtime file storage configuration |
| [**functionConfigGet**](ConfigApi.md#functionconfigget) | **GET** /v1/_/function/config | Get runtime function configuration |
| [**functionConfigUpdate**](ConfigApi.md#functionconfigupdate) | **PUT** /v1/_/function/config | Update runtime function configuration |
| [**imageConfigGet**](ConfigApi.md#imageconfigget) | **GET** /v1/_/image/config | Get runtime image service configuration |
| [**imageConfigUpdate**](ConfigApi.md#imageconfigupdate) | **PUT** /v1/_/image/config | Update runtime image service configuration |
| [**tasksConfigGet**](ConfigApi.md#tasksconfigget) | **GET** /v1/_/tasks/config | Get runtime tasks orchestrator configuration |
| [**tasksConfigUpdate**](ConfigApi.md#tasksconfigupdate) | **PUT** /v1/_/tasks/config | Update runtime tasks orchestrator configuration |



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


## consoleConfigGet

> Config consoleConfigGet()

Get dynamic runtime console configuration

Returns active runtime console configuration settings including session duration, MFA enforcement policy, and allowed origins. Requires console:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { ConsoleConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.consoleConfigGet();
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


## consoleConfigUpdate

> Config consoleConfigUpdate(config)

Update dynamic runtime console configuration

Mutates runtime console configuration parameters in console.config, validating structure and applying settings dynamically across cluster nodes. Requires console:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { ConsoleConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for console.Config
    config: ...,
  } satisfies ConsoleConfigUpdateRequest;

  try {
    const data = await api.consoleConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for console.Config | |

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


## fileStorageConfigGet

> Config fileStorageConfigGet()

Get runtime file storage configuration

Returns dynamic operational configuration settings for the file storage subsystem including global size limits and default backends. Requires storage:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { FileStorageConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.fileStorageConfigGet();
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


## fileStorageConfigUpdate

> Config fileStorageConfigUpdate(config)

Update runtime file storage configuration

Updates dynamic operational configuration for the file storage subsystem in storage.config, tuning global upload limits and backends without restart. Requires storage:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { FileStorageConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for filestorage.Config
    config: ...,
  } satisfies FileStorageConfigUpdateRequest;

  try {
    const data = await api.fileStorageConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for filestorage.Config | |

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


## functionConfigGet

> Config functionConfigGet()

Get runtime function configuration

Returns operational function runtime settings including default isolate timeouts, memory limits, runner concurrency, and isolate pool capacities. Requires function:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { FunctionConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.functionConfigGet();
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


## functionConfigUpdate

> Config functionConfigUpdate(config)

Update runtime function configuration

Modifies operational function runtime settings in function.config, tuning concurrency limits, memory caps, and execution timeouts without restarting the gateway. Requires function:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { FunctionConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for function.Config
    config: ...,
  } satisfies FunctionConfigUpdateRequest;

  try {
    const data = await api.functionConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for function.Config | |

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


## tasksConfigGet

> Config tasksConfigGet()

Get runtime tasks orchestrator configuration

Returns operational task configuration settings including worker concurrency limits, poller interval, default timeout guards, and exponential retry backoff parameters. Requires tasks:config.read scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { TasksConfigGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  try {
    const data = await api.tasksConfigGet();
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


## tasksConfigUpdate

> Config tasksConfigUpdate(config)

Update runtime tasks orchestrator configuration

Updates operational task configuration in tasks.config, dynamically adjusting worker concurrency, polling intervals, and retry backoff limits without restarting workers. Requires tasks:config.write scope.

### Example

```ts
import {
  Configuration,
  ConfigApi,
} from '';
import type { TasksConfigUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigApi();

  const body = {
    // Config | Request body for tasks.Config
    config: ...,
  } satisfies TasksConfigUpdateRequest;

  try {
    const data = await api.tasksConfigUpdate(body);
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
| **config** | [Config](Config.md) | Request body for tasks.Config | |

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

