# UserPasskeysAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserPasskeysDelete**](UserPasskeysAPI.md#authuserpasskeysdelete) | **DELETE** /v1/auth/user/passkeys/{passkey_id} | Revoke a registered passkey credential
[**authUserPasskeysList**](UserPasskeysAPI.md#authuserpasskeyslist) | **GET** /v1/auth/user/passkeys | List registered passkeys for current user


# **authUserPasskeysDelete**
```swift
    open class func authUserPasskeysDelete(passkeyId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke a registered passkey credential

Deletes a registered WebAuthn passkey credential from auth.passkeys belonging to the authenticated user. Prohibits deleting credentials belonging to other users. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let passkeyId = "passkeyId_example" // String | 

// Revoke a registered passkey credential
do {
    let response = try await UserPasskeysAPI.authUserPasskeysDelete(passkeyId: passkeyId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **passkeyId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserPasskeysList**
```swift
    open class func authUserPasskeysList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [Passkey]
```

List registered passkeys for current user

Retrieves all registered WebAuthn / FIDO2 passkey credentials for the authenticated user from auth.passkeys, including credential IDs, friendly names, creation timestamps, and last used dates. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List registered passkeys for current user
do {
    let response = try await UserPasskeysAPI.authUserPasskeysList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**[Passkey]**](Passkey.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

