# SignOutAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authSignOut**](SignOutAPI.md#authsignout) | **POST** /v1/auth/sign-out | Revoke active session and refresh token


# **authSignOut**
```swift
    open class func authSignOut(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke active session and refresh token

Revokes active refresh token, deletes corresponding session record from auth.sessions, clears session cookies, and emits auth.user.signed_out event.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Revoke active session and refresh token
do {
    let response = try await SignOutAPI.authSignOut(body: body)
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

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

