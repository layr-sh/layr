# TokenAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authTokenRefresh**](TokenAPI.md#authtokenrefresh) | **POST** /v1/auth/token/refresh | Exchange refresh token for fresh JWT access token


# **authTokenRefresh**
```swift
    open class func authTokenRefresh(refreshTokenInput: RefreshTokenInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Exchange refresh token for fresh JWT access token

Validates submitted refresh token against SHA-256 hash in auth.sessions, checks session expiry and user lock status, rotates refresh token, and issues a freshly signed Ed25519 JWT access token with updated claims resolved from public.auth_claims.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let refreshTokenInput = RefreshTokenInput(refreshToken: "refreshToken_example") // RefreshTokenInput | Request body for auth.RefreshTokenInput

// Exchange refresh token for fresh JWT access token
do {
    let response = try await TokenAPI.authTokenRefresh(refreshTokenInput: refreshTokenInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **refreshTokenInput** | [**RefreshTokenInput**](RefreshTokenInput.md) | Request body for auth.RefreshTokenInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

