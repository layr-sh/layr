# AnonymousAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authAnonymousSignIn**](AnonymousAPI.md#authanonymoussignin) | **POST** /v1/auth/anonymous | Initialize or sign in an anonymous guest user account


# **authAnonymousSignIn**
```swift
    open class func authAnonymousSignIn(signInAnonymousInput: SignInAnonymousInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Initialize or sign in an anonymous guest user account

Creates an ephemeral guest user account in auth.users with role 'authenticated', is_anonymous set to true, null email/phone, and optional initial profile properties. Publishes auth.user.signed_up event and issues an Ed25519 JWT access token, rotated refresh token in auth.sessions, and authentication cookies.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let signInAnonymousInput = SignInAnonymousInput(properties: "TODO") // SignInAnonymousInput | Request body for auth.SignInAnonymousInput

// Initialize or sign in an anonymous guest user account
do {
    let response = try await AnonymousAPI.authAnonymousSignIn(signInAnonymousInput: signInAnonymousInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **signInAnonymousInput** | [**SignInAnonymousInput**](SignInAnonymousInput.md) | Request body for auth.SignInAnonymousInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

