# URLsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**fileStorageUrlPresign**](URLsAPI.md#filestorageurlpresign) | **POST** /v1/file-storage/url/presign | Mint a presigned capability URL for temporary file access


# **fileStorageUrlPresign**
```swift
    open class func fileStorageUrlPresign(presignURLInput: PresignURLInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> PresignURLResponse
```

Mint a presigned capability URL for temporary file access

Generates a cryptographically signed HMAC-SHA256 capability URL granting temporary read (GET) or write (PUT) access to a specific bucket and key path with configurable expiration TTL.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let presignURLInput = PresignURLInput(bucket: "bucket_example", expiresInSeconds: 123, key: "key_example", operation: "operation_example") // PresignURLInput | Request body for filestorage.PresignURLInput

// Mint a presigned capability URL for temporary file access
do {
    let response = try await URLsAPI.fileStorageUrlPresign(presignURLInput: presignURLInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **presignURLInput** | [**PresignURLInput**](PresignURLInput.md) | Request body for filestorage.PresignURLInput | 

### Return type

[**PresignURLResponse**](PresignURLResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

