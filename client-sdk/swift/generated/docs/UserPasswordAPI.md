# UserPasswordAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserPasswordUpdate**](UserPasswordAPI.md#authuserpasswordupdate) | **PATCH** /v1/auth/user/password | Change or set password for current user


# **authUserPasswordUpdate**
```swift
    open class func authUserPasswordUpdate(updateUserPasswordInput: UpdateUserPasswordInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Change or set password for current user

Updates the account password in auth.users. Verifies current password if one is already set, enforces password complexity and breach checks, and rehashes with Argon2id. Prohibited on anonymous accounts. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let updateUserPasswordInput = UpdateUserPasswordInput(currentPassword: "currentPassword_example", newPassword: "newPassword_example") // UpdateUserPasswordInput | Request body for auth.UpdateUserPasswordInput

// Change or set password for current user
do {
    let response = try await UserPasswordAPI.authUserPasswordUpdate(updateUserPasswordInput: updateUserPasswordInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **updateUserPasswordInput** | [**UpdateUserPasswordInput**](UpdateUserPasswordInput.md) | Request body for auth.UpdateUserPasswordInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

