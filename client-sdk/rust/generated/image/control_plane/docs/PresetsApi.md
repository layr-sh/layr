# \PresetsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**image_presets_create**](PresetsApi.md#image_presets_create) | **POST** /v1/_/image/presets | Create a named image transformation preset
[**image_presets_delete**](PresetsApi.md#image_presets_delete) | **DELETE** /v1/_/image/presets/{preset_id} | Delete an image transformation preset
[**image_presets_get**](PresetsApi.md#image_presets_get) | **GET** /v1/_/image/presets/{preset_id} | Get image transformation preset by ID
[**image_presets_list**](PresetsApi.md#image_presets_list) | **GET** /v1/_/image/presets | List configured image transformation presets
[**image_presets_update**](PresetsApi.md#image_presets_update) | **PUT** /v1/_/image/presets/{preset_id} | Update an image transformation preset



## image_presets_create

> models::Preset image_presets_create(create_preset_input)
Create a named image transformation preset

Creates a new transformation preset in image.presets defining reusable imgproxy processing options (dimensions, crop mode, quality, format). Emits image.preset.created event. Requires image:preset.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_preset_input** | [**CreatePresetInput**](CreatePresetInput.md) | Request body for image.CreatePresetInput | [required] |

### Return type

[**models::Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_presets_delete

> serde_json::Value image_presets_delete(preset_id)
Delete an image transformation preset

Deletes a transformation preset by ID from image.presets. Emits image.preset.deleted event. Requires image:preset.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**preset_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_presets_get

> models::Preset image_presets_get(preset_id)
Get image transformation preset by ID

Retrieves a single transformation preset definition by ID from image.presets. Requires image:preset.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**preset_id** | **String** |  | [required] |

### Return type

[**models::Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_presets_list

> models::ListPresetsResponse image_presets_list()
List configured image transformation presets

Returns all configured transformation presets from image.presets with preset names, processing option strings, and creation timestamps. Requires image:preset.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListPresetsResponse**](ListPresetsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_presets_update

> models::Preset image_presets_update(preset_id, update_preset_input)
Update an image transformation preset

Modifies the transformation options or friendly name of an existing preset in image.presets. Emits image.preset.updated event. Requires image:preset.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**preset_id** | **String** |  | [required] |
**update_preset_input** | [**UpdatePresetInput**](UpdatePresetInput.md) | Request body for image.UpdatePresetInput | [required] |

### Return type

[**models::Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

