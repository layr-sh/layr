# SignUpAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authSignUp**](SignUpAPI.md#authsignup) | **POST** /v1/auth/sign-up | Register a new application user with password


# **authSignUp**
```swift
    open class func authSignUp(signUpInput: SignUpInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Register a new application user with password

Registers a new application user account with email or normalized E.164 phone, password complexity validation, breach detection, and Argon2id key derivation. Supports seamless upgrade of anonymous guest accounts. Dispatches verification code if confirmation is required; otherwise issues Ed25519 JWT tokens.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let signUpInput = SignUpInput(captchaToken: "captchaToken_example", email: "email_example", password: "password_example", phone: "phone_example", properties: "TODO") // SignUpInput | Request body for auth.SignUpInput

// Register a new application user with password
do {
    let response = try await SignUpAPI.authSignUp(signUpInput: signUpInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **signUpInput** | [**SignUpInput**](SignUpInput.md) | Request body for auth.SignUpInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

