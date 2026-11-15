# UserAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserDelete**](UserAPI.md#authuserdelete) | **DELETE** /v1/auth/user | Permanently delete authenticated user account
[**authUserExport**](UserAPI.md#authuserexport) | **POST** /v1/auth/user/export | Export comprehensive user account data graph (GDPR Article 20)
[**authUserGet**](UserAPI.md#authuserget) | **GET** /v1/auth/user | Get profile and verification details of authenticated user


# **authUserDelete**
```swift
    open class func authUserDelete(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Permanently delete authenticated user account

Permanently deletes the authenticated user from auth.users, cascading deletion across auth.sessions, auth.identities, auth.passkeys, and user properties. Clears authentication cookies and emits auth.user.deleted event. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Permanently delete authenticated user account
do {
    let response = try await UserAPI.authUserDelete()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserExport**
```swift
    open class func authUserExport(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ExportUserResponse
```

Export comprehensive user account data graph (GDPR Article 20)

Exports full personal data graph associated with the authenticated user, including profile metadata, linked authentication identities, active sessions, and passkey registrations in portable JSON format. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Export comprehensive user account data graph (GDPR Article 20)
do {
    let response = try await UserAPI.authUserExport(body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**ExportUserResponse**](ExportUserResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserGet**
```swift
    open class func authUserGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetUserResponse
```

Get profile and verification details of authenticated user

Retrieves authenticated user profile from auth.users, including email, phone, verification timestamps, role, custom properties JSON, MFA enrollment status, and lockout flags. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get profile and verification details of authenticated user
do {
    let response = try await UserAPI.authUserGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetUserResponse**](GetUserResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

