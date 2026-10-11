# UserEmailAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserEmailUpdate**](UserEmailAPI.md#authuseremailupdate) | **PATCH** /v1/auth/user/email | Request update of authenticated user email address


# **authUserEmailUpdate**
```swift
    open class func authUserEmailUpdate(updateUserEmailInput: UpdateUserEmailInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request update of authenticated user email address

Asserts uniqueness of the new email address, generates an Argon2id-hashed single-use verification code, and dispatches it to the requested email. The address is updated once verified. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let updateUserEmailInput = UpdateUserEmailInput(email: "email_example") // UpdateUserEmailInput | Request body for auth.UpdateUserEmailInput

// Request update of authenticated user email address
do {
    let response = try await UserEmailAPI.authUserEmailUpdate(updateUserEmailInput: updateUserEmailInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **updateUserEmailInput** | [**UpdateUserEmailInput**](UpdateUserEmailInput.md) | Request body for auth.UpdateUserEmailInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

