# UserPhoneAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserPhoneUpdate**](UserPhoneAPI.md#authuserphoneupdate) | **PATCH** /v1/auth/user/phone | Request update of authenticated user phone number


# **authUserPhoneUpdate**
```swift
    open class func authUserPhoneUpdate(updateUserPhoneInput: UpdateUserPhoneInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request update of authenticated user phone number

Normalizes new phone number to E.164, verifies uniqueness in auth.users, generates single-use verification code, and dispatches via SMS. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let updateUserPhoneInput = UpdateUserPhoneInput(phone: "phone_example") // UpdateUserPhoneInput | Request body for auth.UpdateUserPhoneInput

// Request update of authenticated user phone number
do {
    let response = try await UserPhoneAPI.authUserPhoneUpdate(updateUserPhoneInput: updateUserPhoneInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **updateUserPhoneInput** | [**UpdateUserPhoneInput**](UpdateUserPhoneInput.md) | Request body for auth.UpdateUserPhoneInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

