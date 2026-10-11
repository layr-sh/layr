# KVAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataKvDelete**](KVAPI.md#datakvdelete) | **DELETE** /v1/data/kv/{key} | Delete an ephemeral KV entry
[**dataKvGet**](KVAPI.md#datakvget) | **GET** /v1/data/kv/{key} | Get an ephemeral KV entry by key
[**dataKvIncrement**](KVAPI.md#datakvincrement) | **POST** /v1/data/kv/increment | Atomically increment or decrement an integer counter
[**dataKvMget**](KVAPI.md#datakvmget) | **POST** /v1/data/kv/mget | Batch get multiple ephemeral KV entries
[**dataKvMset**](KVAPI.md#datakvmset) | **POST** /v1/data/kv/mset | Batch set multiple ephemeral KV entries
[**dataKvPut**](KVAPI.md#datakvput) | **PUT** /v1/data/kv/{key} | Put raw ephemeral KV entry with query TTL
[**dataKvSet**](KVAPI.md#datakvset) | **POST** /v1/data/kv/{key} | Set an ephemeral KV entry with JSON envelope
[**dataKvTouch**](KVAPI.md#datakvtouch) | **PATCH** /v1/data/kv/{key} | Refresh TTL of an existing KV entry


# **dataKvDelete**
```swift
    open class func dataKvDelete(key: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete an ephemeral KV entry

Evicts a key-value entry from the distributed KV cache.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let key = "key_example" // String | 

// Delete an ephemeral KV entry
do {
    let response = try await KVAPI.dataKvDelete(key: key)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **key** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvGet**
```swift
    open class func dataKvGet(key: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetKVResponse
```

Get an ephemeral KV entry by key

Retrieves a key-value entry with session/tenant isolation from the distributed KV store. Returns 404 if expired or not found.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let key = "key_example" // String | 

// Get an ephemeral KV entry by key
do {
    let response = try await KVAPI.dataKvGet(key: key)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **key** | **String** |  | 

### Return type

[**GetKVResponse**](GetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvIncrement**
```swift
    open class func dataKvIncrement(incrementKVInput: IncrementKVInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> IncrementKVResponse
```

Atomically increment or decrement an integer counter

Atomically modifies an integer counter in the KV store by step value (positive or negative) with optional TTL preservation, returning the updated integer.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let incrementKVInput = IncrementKVInput(key: "key_example", step: 123, ttl: 123) // IncrementKVInput | Request body for data.IncrementKVInput

// Atomically increment or decrement an integer counter
do {
    let response = try await KVAPI.dataKvIncrement(incrementKVInput: incrementKVInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **incrementKVInput** | [**IncrementKVInput**](IncrementKVInput.md) | Request body for data.IncrementKVInput | 

### Return type

[**IncrementKVResponse**](IncrementKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvMget**
```swift
    open class func dataKvMget(getMultipleKVInput: GetMultipleKVInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetMultipleKVResponse
```

Batch get multiple ephemeral KV entries

Fetches values for an array of keys from the distributed ephemeral KV cache in a single atomic pipelined round-trip, maintaining tenant/session key isolation.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let getMultipleKVInput = GetMultipleKVInput(keys: ["keys_example"]) // GetMultipleKVInput | Request body for data.GetMultipleKVInput

// Batch get multiple ephemeral KV entries
do {
    let response = try await KVAPI.dataKvMget(getMultipleKVInput: getMultipleKVInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **getMultipleKVInput** | [**GetMultipleKVInput**](GetMultipleKVInput.md) | Request body for data.GetMultipleKVInput | 

### Return type

[**GetMultipleKVResponse**](GetMultipleKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvMset**
```swift
    open class func dataKvMset(setMultipleKVInput: SetMultipleKVInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SetMultipleKVResponse
```

Batch set multiple ephemeral KV entries

Stores multiple key-value pairs with optional individual TTLs in the distributed KV cache in a single atomic round-trip.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let setMultipleKVInput = SetMultipleKVInput(entries: "TODO", ttl: 123) // SetMultipleKVInput | Request body for data.SetMultipleKVInput

// Batch set multiple ephemeral KV entries
do {
    let response = try await KVAPI.dataKvMset(setMultipleKVInput: setMultipleKVInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **setMultipleKVInput** | [**SetMultipleKVInput**](SetMultipleKVInput.md) | Request body for data.SetMultipleKVInput | 

### Return type

[**SetMultipleKVResponse**](SetMultipleKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvPut**
```swift
    open class func dataKvPut(key: String, body: String, ttl: Int? = nil, nx: Bool? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SetKVResponse
```

Put raw ephemeral KV entry with query TTL

Stores raw request body directly in the KV store with optional ?ttl= query parameter and optional set_nx conditional insertion flag.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let key = "key_example" // String | 
let body = "body_example" // String | Request body for string
let ttl = 987 // Int | Key time-to-live expiration in seconds (optional)
let nx = true // Bool | Only set the key if it does not already exist (optional)

// Put raw ephemeral KV entry with query TTL
do {
    let response = try await KVAPI.dataKvPut(key: key, body: body, ttl: ttl, nx: nx)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **key** | **String** |  | 
 **body** | **String** | Request body for string | 
 **ttl** | **Int** | Key time-to-live expiration in seconds | [optional] 
 **nx** | **Bool** | Only set the key if it does not already exist | [optional] 

### Return type

[**SetKVResponse**](SetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvSet**
```swift
    open class func dataKvSet(key: String, setKVInput: SetKVInput, nx: Bool? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SetKVResponse
```

Set an ephemeral KV entry with JSON envelope

Stores a value in the KV cache using a { value, ttl } JSON envelope with optional set_nx conditional insertion flag (maximum payload 2 MB).

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let key = "key_example" // String | 
let setKVInput = SetKVInput(ttl: 123, value: "value_example") // SetKVInput | Request body for data.SetKVInput
let nx = true // Bool | Only set the key if it does not already exist (optional)

// Set an ephemeral KV entry with JSON envelope
do {
    let response = try await KVAPI.dataKvSet(key: key, setKVInput: setKVInput, nx: nx)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **key** | **String** |  | 
 **setKVInput** | [**SetKVInput**](SetKVInput.md) | Request body for data.SetKVInput | 
 **nx** | **Bool** | Only set the key if it does not already exist | [optional] 

### Return type

[**SetKVResponse**](SetKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataKvTouch**
```swift
    open class func dataKvTouch(key: String, touchKVInput: TouchKVInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> TouchKVResponse
```

Refresh TTL of an existing KV entry

Updates the expiration TTL of an existing key in the KV cache without altering its stored payload.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let key = "key_example" // String | 
let touchKVInput = TouchKVInput(ttl: 123) // TouchKVInput | Request body for data.TouchKVInput

// Refresh TTL of an existing KV entry
do {
    let response = try await KVAPI.dataKvTouch(key: key, touchKVInput: touchKVInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **key** | **String** |  | 
 **touchKVInput** | [**TouchKVInput**](TouchKVInput.md) | Request body for data.TouchKVInput | 

### Return type

[**TouchKVResponse**](TouchKVResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

