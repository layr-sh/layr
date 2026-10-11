# InfoAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**imageInfoGet**](InfoAPI.md#imageinfoget) | **GET** /v1/image/info/{signature}/{path...} | Inspect image metadata by path
[**imageInfoPost**](InfoAPI.md#imageinfopost) | **POST** /v1/image/info | Inspect image metadata from binary upload


# **imageInfoGet**
```swift
    open class func imageInfoGet(signature: String, path: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetInfoResponse
```

Inspect image metadata by path

Extracts structural metadata (width, height, format, file size, colorspace, EXIF, orientation) from a remote or storage image path using cryptographic signature verification.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let signature = "signature_example" // String | 
let path = "path_example" // String |  (might contain slashes)

// Inspect image metadata by path
do {
    let response = try await InfoAPI.imageInfoGet(signature: signature, path: path)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **signature** | **String** |  | 
 **path** | **String** |  (might contain slashes) | 

### Return type

[**GetInfoResponse**](GetInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imageInfoPost**
```swift
    open class func imageInfoPost(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetInfoResponse
```

Inspect image metadata from binary upload

Probes and extracts dimensions, format, file size, colorspace, and EXIF metadata from an uploaded image binary payload.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Inspect image metadata from binary upload
do {
    let response = try await InfoAPI.imageInfoPost(body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**GetInfoResponse**](GetInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

