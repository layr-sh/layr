# UserEmailVerificationAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserEmailVerificationConfirm**](UserEmailVerificationAPI.md#authuseremailverificationconfirm) | **POST** /v1/auth/user/email/verification/confirm | Confirm email verification code
[**authUserEmailVerificationRequest**](UserEmailVerificationAPI.md#authuseremailverificationrequest) | **POST** /v1/auth/user/email/verification/request | Request an email verification code for current email


# **authUserEmailVerificationConfirm**
```swift
    open class func authUserEmailVerificationConfirm(confirmEmailVerificationInput: ConfirmEmailVerificationInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Confirm email verification code

Verifies submitted single-use code against pending email verification record, sets email_verified_at timestamp in auth.users, and updates primary email if part of an address change flow. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let confirmEmailVerificationInput = ConfirmEmailVerificationInput(code: "code_example", email: "email_example") // ConfirmEmailVerificationInput | Request body for auth.ConfirmEmailVerificationInput

// Confirm email verification code
do {
    let response = try await UserEmailVerificationAPI.authUserEmailVerificationConfirm(confirmEmailVerificationInput: confirmEmailVerificationInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **confirmEmailVerificationInput** | [**ConfirmEmailVerificationInput**](ConfirmEmailVerificationInput.md) | Request body for auth.ConfirmEmailVerificationInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserEmailVerificationRequest**
```swift
    open class func authUserEmailVerificationRequest(requestEmailVerificationInput: RequestEmailVerificationInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request an email verification code for current email

Generates and dispatches a single-use verification code to the authenticated user's registered unverified email address. Rate-limited by IP and user cooldowns. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let requestEmailVerificationInput = RequestEmailVerificationInput(email: "email_example") // RequestEmailVerificationInput | Request body for auth.RequestEmailVerificationInput

// Request an email verification code for current email
do {
    let response = try await UserEmailVerificationAPI.authUserEmailVerificationRequest(requestEmailVerificationInput: requestEmailVerificationInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **requestEmailVerificationInput** | [**RequestEmailVerificationInput**](RequestEmailVerificationInput.md) | Request body for auth.RequestEmailVerificationInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

