# PresetsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**imagePresetsCreate**](PresetsAPI.md#imagepresetscreate) | **POST** /v1/_/image/presets | Create a named image transformation preset
[**imagePresetsDelete**](PresetsAPI.md#imagepresetsdelete) | **DELETE** /v1/_/image/presets/{preset_id} | Delete an image transformation preset
[**imagePresetsGet**](PresetsAPI.md#imagepresetsget) | **GET** /v1/_/image/presets/{preset_id} | Get image transformation preset by ID
[**imagePresetsList**](PresetsAPI.md#imagepresetslist) | **GET** /v1/_/image/presets | List configured image transformation presets
[**imagePresetsUpdate**](PresetsAPI.md#imagepresetsupdate) | **PUT** /v1/_/image/presets/{preset_id} | Update an image transformation preset


# **imagePresetsCreate**
```swift
    open class func imagePresetsCreate(createPresetInput: CreatePresetInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Preset
```

Create a named image transformation preset

Creates a new transformation preset in image.presets defining reusable imgproxy processing options (dimensions, crop mode, quality, format). Emits image.preset.created event. Requires image:preset.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createPresetInput = CreatePresetInput(description: "description_example", name: "name_example", processingOptions: "processingOptions_example") // CreatePresetInput | Request body for image.CreatePresetInput

// Create a named image transformation preset
do {
    let response = try await PresetsAPI.imagePresetsCreate(createPresetInput: createPresetInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createPresetInput** | [**CreatePresetInput**](CreatePresetInput.md) | Request body for image.CreatePresetInput | 

### Return type

[**Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imagePresetsDelete**
```swift
    open class func imagePresetsDelete(presetId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete an image transformation preset

Deletes a transformation preset by ID from image.presets. Emits image.preset.deleted event. Requires image:preset.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let presetId = "presetId_example" // String | 

// Delete an image transformation preset
do {
    let response = try await PresetsAPI.imagePresetsDelete(presetId: presetId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **presetId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imagePresetsGet**
```swift
    open class func imagePresetsGet(presetId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Preset
```

Get image transformation preset by ID

Retrieves a single transformation preset definition by ID from image.presets. Requires image:preset.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let presetId = "presetId_example" // String | 

// Get image transformation preset by ID
do {
    let response = try await PresetsAPI.imagePresetsGet(presetId: presetId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **presetId** | **String** |  | 

### Return type

[**Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imagePresetsList**
```swift
    open class func imagePresetsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListPresetsResponse
```

List configured image transformation presets

Returns all configured transformation presets from image.presets with preset names, processing option strings, and creation timestamps. Requires image:preset.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List configured image transformation presets
do {
    let response = try await PresetsAPI.imagePresetsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListPresetsResponse**](ListPresetsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imagePresetsUpdate**
```swift
    open class func imagePresetsUpdate(presetId: String, updatePresetInput: UpdatePresetInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Preset
```

Update an image transformation preset

Modifies the transformation options or friendly name of an existing preset in image.presets. Emits image.preset.updated event. Requires image:preset.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let presetId = "presetId_example" // String | 
let updatePresetInput = UpdatePresetInput(description: "description_example", name: "name_example", processingOptions: "processingOptions_example") // UpdatePresetInput | Request body for image.UpdatePresetInput

// Update an image transformation preset
do {
    let response = try await PresetsAPI.imagePresetsUpdate(presetId: presetId, updatePresetInput: updatePresetInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **presetId** | **String** |  | 
 **updatePresetInput** | [**UpdatePresetInput**](UpdatePresetInput.md) | Request body for image.UpdatePresetInput | 

### Return type

[**Preset**](Preset.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

