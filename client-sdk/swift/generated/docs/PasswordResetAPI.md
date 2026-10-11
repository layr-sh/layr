# PasswordResetAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authPasswordResetConfirm**](PasswordResetAPI.md#authpasswordresetconfirm) | **POST** /v1/auth/password-reset/confirm | Confirm password reset with verification code and new password
[**authPasswordResetRequest**](PasswordResetAPI.md#authpasswordresetrequest) | **POST** /v1/auth/password-reset/request | Request a password reset verification code via email or SMS


# **authPasswordResetConfirm**
```swift
    open class func authPasswordResetConfirm(confirmPasswordResetInput: ConfirmPasswordResetInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Confirm password reset with verification code and new password

Verifies single-use password recovery verification code, enforces password complexity and breach checks, updates password hash using Argon2id key derivation in auth.users, and revokes existing sessions.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let confirmPasswordResetInput = ConfirmPasswordResetInput(code: "code_example", email: "email_example", password: "password_example", phone: "phone_example") // ConfirmPasswordResetInput | Request body for auth.ConfirmPasswordResetInput

// Confirm password reset with verification code and new password
do {
    let response = try await PasswordResetAPI.authPasswordResetConfirm(confirmPasswordResetInput: confirmPasswordResetInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **confirmPasswordResetInput** | [**ConfirmPasswordResetInput**](ConfirmPasswordResetInput.md) | Request body for auth.ConfirmPasswordResetInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authPasswordResetRequest**
```swift
    open class func authPasswordResetRequest(requestPasswordResetInput: RequestPasswordResetInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request a password reset verification code via email or SMS

Initiates password recovery for registered user, generating an Argon2id-hashed single-use verification code with cooldown guards and dispatching it to registered email or SMS destination.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let requestPasswordResetInput = RequestPasswordResetInput(captchaToken: "captchaToken_example", email: "email_example", phone: "phone_example") // RequestPasswordResetInput | Request body for auth.RequestPasswordResetInput

// Request a password reset verification code via email or SMS
do {
    let response = try await PasswordResetAPI.authPasswordResetRequest(requestPasswordResetInput: requestPasswordResetInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **requestPasswordResetInput** | [**RequestPasswordResetInput**](RequestPasswordResetInput.md) | Request body for auth.RequestPasswordResetInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

