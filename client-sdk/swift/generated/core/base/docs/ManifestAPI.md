# ManifestAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreManifest**](ManifestAPI.md#coremanifest) | **GET** /v1/manifest | Get dynamic cluster manifest and enabled services


# **coreManifest**
```swift
    open class func coreManifest(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetManifestResponse
```

Get dynamic cluster manifest and enabled services

Returns cluster metadata, list of enabled platform services, public server base URL, and the derived publishable key required for SDK client initialization. Unrestricted public endpoint.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get dynamic cluster manifest and enabled services
do {
    let response = try await ManifestAPI.coreManifest()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetManifestResponse**](GetManifestResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

