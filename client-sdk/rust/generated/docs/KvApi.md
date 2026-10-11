# \KvApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_kv_delete**](KvApi.md#data_kv_delete) | **DELETE** /v1/data/kv/{key} | Delete an ephemeral KV entry
[**data_kv_get**](KvApi.md#data_kv_get) | **GET** /v1/data/kv/{key} | Get an ephemeral KV entry by key
[**data_kv_increment**](KvApi.md#data_kv_increment) | **POST** /v1/data/kv/increment | Atomically increment or decrement an integer counter
[**data_kv_mget**](KvApi.md#data_kv_mget) | **POST** /v1/data/kv/mget | Batch get multiple ephemeral KV entries
[**data_kv_mset**](KvApi.md#data_kv_mset) | **POST** /v1/data/kv/mset | Batch set multiple ephemeral KV entries
[**data_kv_put**](KvApi.md#data_kv_put) | **PUT** /v1/data/kv/{key} | Put raw ephemeral KV entry with query TTL
[**data_kv_set**](KvApi.md#data_kv_set) | **POST** /v1/data/kv/{key} | Set an ephemeral KV entry with JSON envelope
[**data_kv_touch**](KvApi.md#data_kv_touch) | **PATCH** /v1/data/kv/{key} | Refresh TTL of an existing KV entry



## data_kv_delete

> serde_json::Value data_kv_delete(key)
Delete an ephemeral KV entry

Evicts a key-value entry from the distributed KV cache.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**key** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_get

> models::GetKvResponse data_kv_get(key)
Get an ephemeral KV entry by key

Retrieves a key-value entry with session/tenant isolation from the distributed KV store. Returns 404 if expired or not found.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**key** | **String** |  | [required] |

### Return type

[**models::GetKvResponse**](GetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_increment

> models::IncrementKvResponse data_kv_increment(increment_kv_input)
Atomically increment or decrement an integer counter

Atomically modifies an integer counter in the KV store by step value (positive or negative) with optional TTL preservation, returning the updated integer.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**increment_kv_input** | [**IncrementKvInput**](IncrementKvInput.md) | Request body for data.IncrementKVInput | [required] |

### Return type

[**models::IncrementKvResponse**](IncrementKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_mget

> models::GetMultipleKvResponse data_kv_mget(get_multiple_kv_input)
Batch get multiple ephemeral KV entries

Fetches values for an array of keys from the distributed ephemeral KV cache in a single atomic pipelined round-trip, maintaining tenant/session key isolation.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**get_multiple_kv_input** | [**GetMultipleKvInput**](GetMultipleKvInput.md) | Request body for data.GetMultipleKVInput | [required] |

### Return type

[**models::GetMultipleKvResponse**](GetMultipleKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_mset

> models::SetMultipleKvResponse data_kv_mset(set_multiple_kv_input)
Batch set multiple ephemeral KV entries

Stores multiple key-value pairs with optional individual TTLs in the distributed KV cache in a single atomic round-trip.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**set_multiple_kv_input** | [**SetMultipleKvInput**](SetMultipleKvInput.md) | Request body for data.SetMultipleKVInput | [required] |

### Return type

[**models::SetMultipleKvResponse**](SetMultipleKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_put

> models::SetKvResponse data_kv_put(key, body, ttl, nx)
Put raw ephemeral KV entry with query TTL

Stores raw request body directly in the KV store with optional ?ttl= query parameter and optional set_nx conditional insertion flag.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**key** | **String** |  | [required] |
**body** | **String** | Request body for string | [required] |
**ttl** | Option<**i32**> | Key time-to-live expiration in seconds |  |
**nx** | Option<**bool**> | Only set the key if it does not already exist |  |

### Return type

[**models::SetKvResponse**](SetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_set

> models::SetKvResponse data_kv_set(key, set_kv_input, nx)
Set an ephemeral KV entry with JSON envelope

Stores a value in the KV cache using a { value, ttl } JSON envelope with optional set_nx conditional insertion flag (maximum payload 2 MB).

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**key** | **String** |  | [required] |
**set_kv_input** | [**SetKvInput**](SetKvInput.md) | Request body for data.SetKVInput | [required] |
**nx** | Option<**bool**> | Only set the key if it does not already exist |  |

### Return type

[**models::SetKvResponse**](SetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_kv_touch

> models::TouchKvResponse data_kv_touch(key, touch_kv_input)
Refresh TTL of an existing KV entry

Updates the expiration TTL of an existing key in the KV cache without altering its stored payload.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**key** | **String** |  | [required] |
**touch_kv_input** | [**TouchKvInput**](TouchKvInput.md) | Request body for data.TouchKVInput | [required] |

### Return type

[**models::TouchKvResponse**](TouchKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

