# KVApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataKvDelete**](KVApi.md#datakvdelete) | **DELETE** /v1/data/kv/{key} | Delete an ephemeral KV entry |
| [**dataKvGet**](KVApi.md#datakvget) | **GET** /v1/data/kv/{key} | Get an ephemeral KV entry by key |
| [**dataKvIncrement**](KVApi.md#datakvincrement) | **POST** /v1/data/kv/increment | Atomically increment or decrement an integer counter |
| [**dataKvMget**](KVApi.md#datakvmget) | **POST** /v1/data/kv/mget | Batch get multiple ephemeral KV entries |
| [**dataKvMset**](KVApi.md#datakvmset) | **POST** /v1/data/kv/mset | Batch set multiple ephemeral KV entries |
| [**dataKvPut**](KVApi.md#datakvput) | **PUT** /v1/data/kv/{key} | Put raw ephemeral KV entry with query TTL |
| [**dataKvSet**](KVApi.md#datakvset) | **POST** /v1/data/kv/{key} | Set an ephemeral KV entry with JSON envelope |
| [**dataKvTouch**](KVApi.md#datakvtouch) | **PATCH** /v1/data/kv/{key} | Refresh TTL of an existing KV entry |



## dataKvDelete

> any dataKvDelete(key)

Delete an ephemeral KV entry

Evicts a key-value entry from the distributed KV cache.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // string
    key: key_example,
  } satisfies DataKvDeleteRequest;

  try {
    const data = await api.dataKvDelete(body);
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
| **key** | `string` |  | [Defaults to `undefined`] |

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
| **204** | KV entry deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataKvGet

> GetKVResponse dataKvGet(key)

Get an ephemeral KV entry by key

Retrieves a key-value entry with session/tenant isolation from the distributed KV store. Returns 404 if expired or not found.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // string
    key: key_example,
  } satisfies DataKvGetRequest;

  try {
    const data = await api.dataKvGet(body);
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
| **key** | `string` |  | [Defaults to `undefined`] |

### Return type

[**GetKVResponse**](GetKVResponse.md)

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


## dataKvIncrement

> IncrementKVResponse dataKvIncrement(incrementKVInput)

Atomically increment or decrement an integer counter

Atomically modifies an integer counter in the KV store by step value (positive or negative) with optional TTL preservation, returning the updated integer.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvIncrementRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // IncrementKVInput | Request body for data.IncrementKVInput
    incrementKVInput: ...,
  } satisfies DataKvIncrementRequest;

  try {
    const data = await api.dataKvIncrement(body);
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
| **incrementKVInput** | [IncrementKVInput](IncrementKVInput.md) | Request body for data.IncrementKVInput | |

### Return type

[**IncrementKVResponse**](IncrementKVResponse.md)

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


## dataKvMget

> GetMultipleKVResponse dataKvMget(getMultipleKVInput)

Batch get multiple ephemeral KV entries

Fetches values for an array of keys from the distributed ephemeral KV cache in a single atomic pipelined round-trip, maintaining tenant/session key isolation.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvMgetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // GetMultipleKVInput | Request body for data.GetMultipleKVInput
    getMultipleKVInput: ...,
  } satisfies DataKvMgetRequest;

  try {
    const data = await api.dataKvMget(body);
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
| **getMultipleKVInput** | [GetMultipleKVInput](GetMultipleKVInput.md) | Request body for data.GetMultipleKVInput | |

### Return type

[**GetMultipleKVResponse**](GetMultipleKVResponse.md)

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


## dataKvMset

> SetMultipleKVResponse dataKvMset(setMultipleKVInput)

Batch set multiple ephemeral KV entries

Stores multiple key-value pairs with optional individual TTLs in the distributed KV cache in a single atomic round-trip.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvMsetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // SetMultipleKVInput | Request body for data.SetMultipleKVInput
    setMultipleKVInput: ...,
  } satisfies DataKvMsetRequest;

  try {
    const data = await api.dataKvMset(body);
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
| **setMultipleKVInput** | [SetMultipleKVInput](SetMultipleKVInput.md) | Request body for data.SetMultipleKVInput | |

### Return type

[**SetMultipleKVResponse**](SetMultipleKVResponse.md)

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


## dataKvPut

> SetKVResponse dataKvPut(key, body, ttl, nx)

Put raw ephemeral KV entry with query TTL

Stores raw request body directly in the KV store with optional ?ttl&#x3D; query parameter and optional set_nx conditional insertion flag.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvPutRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // string
    key: key_example,
    // string | Request body for string
    body: body_example,
    // number | Key time-to-live expiration in seconds (optional)
    ttl: 56,
    // boolean | Only set the key if it does not already exist (optional)
    nx: true,
  } satisfies DataKvPutRequest;

  try {
    const data = await api.dataKvPut(body);
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
| **key** | `string` |  | [Defaults to `undefined`] |
| **body** | `string` | Request body for string | |
| **ttl** | `number` | Key time-to-live expiration in seconds | [Optional] [Defaults to `undefined`] |
| **nx** | `boolean` | Only set the key if it does not already exist | [Optional] [Defaults to `undefined`] |

### Return type

[**SetKVResponse**](SetKVResponse.md)

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


## dataKvSet

> SetKVResponse dataKvSet(key, setKVInput, nx)

Set an ephemeral KV entry with JSON envelope

Stores a value in the KV cache using a { value, ttl } JSON envelope with optional set_nx conditional insertion flag (maximum payload 2 MB).

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvSetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // string
    key: key_example,
    // SetKVInput | Request body for data.SetKVInput
    setKVInput: ...,
    // boolean | Only set the key if it does not already exist (optional)
    nx: true,
  } satisfies DataKvSetRequest;

  try {
    const data = await api.dataKvSet(body);
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
| **key** | `string` |  | [Defaults to `undefined`] |
| **setKVInput** | [SetKVInput](SetKVInput.md) | Request body for data.SetKVInput | |
| **nx** | `boolean` | Only set the key if it does not already exist | [Optional] [Defaults to `undefined`] |

### Return type

[**SetKVResponse**](SetKVResponse.md)

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


## dataKvTouch

> TouchKVResponse dataKvTouch(key, touchKVInput)

Refresh TTL of an existing KV entry

Updates the expiration TTL of an existing key in the KV cache without altering its stored payload.

### Example

```ts
import {
  Configuration,
  KVApi,
} from '';
import type { DataKvTouchRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new KVApi();

  const body = {
    // string
    key: key_example,
    // TouchKVInput | Request body for data.TouchKVInput
    touchKVInput: ...,
  } satisfies DataKvTouchRequest;

  try {
    const data = await api.dataKvTouch(body);
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
| **key** | `string` |  | [Defaults to `undefined`] |
| **touchKVInput** | [TouchKVInput](TouchKVInput.md) | Request body for data.TouchKVInput | |

### Return type

[**TouchKVResponse**](TouchKVResponse.md)

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

