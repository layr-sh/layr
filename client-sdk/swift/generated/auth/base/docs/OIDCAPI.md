# OIDCAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authOidcToken**](OIDCAPI.md#authoidctoken) | **POST** /v1/auth/oauth/token | Exchange authorization code or refresh token for OpenID Connect tokens
[**authOidcUserinfo**](OIDCAPI.md#authoidcuserinfo) | **GET** /v1/auth/oauth/userinfo | Retrieve OpenID Connect Core 1.0 user claims


# **authOidcToken**
```swift
    open class func authOidcToken(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> IssueOIDCTokenResponse
```

Exchange authorization code or refresh token for OpenID Connect tokens

Exchanges an OAuth2 / OIDC authorization code (with PKCE code_verifier challenge verification) or refresh token for an Ed25519 JWT ID token, access token, and refresh token according to OpenID Connect Core 1.0 specifications.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Exchange authorization code or refresh token for OpenID Connect tokens
do {
    let response = try await OIDCAPI.authOidcToken(body: body)
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

[**IssueOIDCTokenResponse**](IssueOIDCTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authOidcUserinfo**
```swift
    open class func authOidcUserinfo(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> GetOIDCUserInfoResponse
```

Retrieve OpenID Connect Core 1.0 user claims

Retrieves standard OpenID Connect profile claims (sub, email, email_verified, phone, phone_verified, name) for the authenticated caller bearing a valid Bearer access token.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Retrieve OpenID Connect Core 1.0 user claims
do {
    let response = try await OIDCAPI.authOidcUserinfo()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**GetOIDCUserInfoResponse**](GetOIDCUserInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

