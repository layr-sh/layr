# SignInAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authSignIn**](SignInAPI.md#authsignin) | **POST** /v1/auth/sign-in | Authenticate with email or phone and password


# **authSignIn**
```swift
    open class func authSignIn(signInInput: SignInInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Authenticate with email or phone and password

Authenticates application user with email or normalized E.164 phone and password. Verifies Argon2id hash, checks account lockout status and breach flags. If MFA is enrolled, returns an ephemeral mfa_ticket challenge; otherwise issues Ed25519 JWT access token, refresh token in auth.sessions, and session cookies.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let signInInput = SignInInput(captchaToken: "captchaToken_example", email: "email_example", password: "password_example", phone: "phone_example") // SignInInput | Request body for auth.SignInInput

// Authenticate with email or phone and password
do {
    let response = try await SignInAPI.authSignIn(signInInput: signInInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **signInInput** | [**SignInInput**](SignInInput.md) | Request body for auth.SignInInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

