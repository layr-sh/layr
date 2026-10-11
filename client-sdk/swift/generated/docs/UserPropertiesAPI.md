# UserPropertiesAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserPropertiesUpdate**](UserPropertiesAPI.md#authuserpropertiesupdate) | **PATCH** /v1/auth/user/properties | Update custom personal properties on user profile


# **authUserPropertiesUpdate**
```swift
    open class func authUserPropertiesUpdate(updateUserPropertiesInput: UpdateUserPropertiesInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> UpdateUserPropertiesResponse
```

Update custom personal properties on user profile

Performs shallow merge of custom JSON properties into the properties column of auth.users for the authenticated user, validating JSON size limits. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let updateUserPropertiesInput = UpdateUserPropertiesInput(properties: "TODO") // UpdateUserPropertiesInput | Request body for auth.UpdateUserPropertiesInput

// Update custom personal properties on user profile
do {
    let response = try await UserPropertiesAPI.authUserPropertiesUpdate(updateUserPropertiesInput: updateUserPropertiesInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **updateUserPropertiesInput** | [**UpdateUserPropertiesInput**](UpdateUserPropertiesInput.md) | Request body for auth.UpdateUserPropertiesInput | 

### Return type

[**UpdateUserPropertiesResponse**](UpdateUserPropertiesResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

