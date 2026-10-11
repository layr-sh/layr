# UserPhoneVerificationAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserPhoneVerificationConfirm**](UserPhoneVerificationAPI.md#authuserphoneverificationconfirm) | **POST** /v1/auth/user/phone/verification/confirm | Confirm phone verification code
[**authUserPhoneVerificationRequest**](UserPhoneVerificationAPI.md#authuserphoneverificationrequest) | **POST** /v1/auth/user/phone/verification/request | Request a phone verification code for current phone


# **authUserPhoneVerificationConfirm**
```swift
    open class func authUserPhoneVerificationConfirm(confirmPhoneVerificationInput: ConfirmPhoneVerificationInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Confirm phone verification code

Verifies submitted single-use SMS code against pending verification record, updates phone_verified_at in auth.users, and marks phone as verified. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let confirmPhoneVerificationInput = ConfirmPhoneVerificationInput(code: "code_example", phone: "phone_example") // ConfirmPhoneVerificationInput | Request body for auth.ConfirmPhoneVerificationInput

// Confirm phone verification code
do {
    let response = try await UserPhoneVerificationAPI.authUserPhoneVerificationConfirm(confirmPhoneVerificationInput: confirmPhoneVerificationInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **confirmPhoneVerificationInput** | [**ConfirmPhoneVerificationInput**](ConfirmPhoneVerificationInput.md) | Request body for auth.ConfirmPhoneVerificationInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserPhoneVerificationRequest**
```swift
    open class func authUserPhoneVerificationRequest(requestPhoneVerificationInput: RequestPhoneVerificationInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request a phone verification code for current phone

Generates and dispatches a single-use verification code via SMS to the authenticated user's registered phone number. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let requestPhoneVerificationInput = RequestPhoneVerificationInput(phone: "phone_example") // RequestPhoneVerificationInput | Request body for auth.RequestPhoneVerificationInput

// Request a phone verification code for current phone
do {
    let response = try await UserPhoneVerificationAPI.authUserPhoneVerificationRequest(requestPhoneVerificationInput: requestPhoneVerificationInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **requestPhoneVerificationInput** | [**RequestPhoneVerificationInput**](RequestPhoneVerificationInput.md) | Request body for auth.RequestPhoneVerificationInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

