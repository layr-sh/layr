# TransformAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**imageTransformGet**](TransformAPI.md#imagetransformget) | **GET** /v1/image/{signature}/{path...} | Transform and optimize an image dynamically


# **imageTransformGet**
```swift
    open class func imageTransformGet(signature: String, path: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> URL
```

Transform and optimize an image dynamically

Dynamically fetches, crops, resizes, optimizes, and transforms source images according to imgproxy option specifications. Verifies HMAC-SHA256 signature if enabled, caches transformed result, and streams binary image with appropriate Content-Type header.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let signature = "signature_example" // String | 
let path = "path_example" // String |  (might contain slashes)

// Transform and optimize an image dynamically
do {
    let response = try await TransformAPI.imageTransformGet(signature: signature, path: path)
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

**URL**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/octet-stream

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

