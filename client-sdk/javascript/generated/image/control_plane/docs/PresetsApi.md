# PresetsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**imagePresetsCreate**](PresetsApi.md#imagepresetscreate) | **POST** /v1/_/image/presets | Create a named image transformation preset |
| [**imagePresetsDelete**](PresetsApi.md#imagepresetsdelete) | **DELETE** /v1/_/image/presets/{preset_id} | Delete an image transformation preset |
| [**imagePresetsGet**](PresetsApi.md#imagepresetsget) | **GET** /v1/_/image/presets/{preset_id} | Get image transformation preset by ID |
| [**imagePresetsList**](PresetsApi.md#imagepresetslist) | **GET** /v1/_/image/presets | List configured image transformation presets |
| [**imagePresetsUpdate**](PresetsApi.md#imagepresetsupdate) | **PUT** /v1/_/image/presets/{preset_id} | Update an image transformation preset |



## imagePresetsCreate

> Preset imagePresetsCreate(createPresetInput)

Create a named image transformation preset

Creates a new transformation preset in image.presets defining reusable imgproxy processing options (dimensions, crop mode, quality, format). Emits image.preset.created event. Requires image:preset.write scope.

### Example

```ts
import {
  Configuration,
  PresetsApi,
} from '';
import type { ImagePresetsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PresetsApi();

  const body = {
    // CreatePresetInput | Request body for image.CreatePresetInput
    createPresetInput: ...,
  } satisfies ImagePresetsCreateRequest;

  try {
    const data = await api.imagePresetsCreate(body);
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
| **createPresetInput** | [CreatePresetInput](CreatePresetInput.md) | Request body for image.CreatePresetInput | |

### Return type

[**Preset**](Preset.md)

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


## imagePresetsDelete

> any imagePresetsDelete(presetId)

Delete an image transformation preset

Deletes a transformation preset by ID from image.presets. Emits image.preset.deleted event. Requires image:preset.write scope.

### Example

```ts
import {
  Configuration,
  PresetsApi,
} from '';
import type { ImagePresetsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PresetsApi();

  const body = {
    // string
    presetId: presetId_example,
  } satisfies ImagePresetsDeleteRequest;

  try {
    const data = await api.imagePresetsDelete(body);
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
| **presetId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Preset deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## imagePresetsGet

> Preset imagePresetsGet(presetId)

Get image transformation preset by ID

Retrieves a single transformation preset definition by ID from image.presets. Requires image:preset.read scope.

### Example

```ts
import {
  Configuration,
  PresetsApi,
} from '';
import type { ImagePresetsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PresetsApi();

  const body = {
    // string
    presetId: presetId_example,
  } satisfies ImagePresetsGetRequest;

  try {
    const data = await api.imagePresetsGet(body);
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
| **presetId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Preset**](Preset.md)

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


## imagePresetsList

> ListPresetsResponse imagePresetsList()

List configured image transformation presets

Returns all configured transformation presets from image.presets with preset names, processing option strings, and creation timestamps. Requires image:preset.read scope.

### Example

```ts
import {
  Configuration,
  PresetsApi,
} from '';
import type { ImagePresetsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PresetsApi();

  try {
    const data = await api.imagePresetsList();
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

[**ListPresetsResponse**](ListPresetsResponse.md)

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


## imagePresetsUpdate

> Preset imagePresetsUpdate(presetId, updatePresetInput)

Update an image transformation preset

Modifies the transformation options or friendly name of an existing preset in image.presets. Emits image.preset.updated event. Requires image:preset.write scope.

### Example

```ts
import {
  Configuration,
  PresetsApi,
} from '';
import type { ImagePresetsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PresetsApi();

  const body = {
    // string
    presetId: presetId_example,
    // UpdatePresetInput | Request body for image.UpdatePresetInput
    updatePresetInput: ...,
  } satisfies ImagePresetsUpdateRequest;

  try {
    const data = await api.imagePresetsUpdate(body);
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
| **presetId** | `string` |  | [Defaults to `undefined`] |
| **updatePresetInput** | [UpdatePresetInput](UpdatePresetInput.md) | Request body for image.UpdatePresetInput | |

### Return type

[**Preset**](Preset.md)

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

