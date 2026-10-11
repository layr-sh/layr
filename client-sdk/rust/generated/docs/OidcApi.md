# \OidcApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_oidc_token**](OidcApi.md#auth_oidc_token) | **POST** /v1/auth/oauth/token | Exchange authorization code or refresh token for OpenID Connect tokens
[**auth_oidc_userinfo**](OidcApi.md#auth_oidc_userinfo) | **GET** /v1/auth/oauth/userinfo | Retrieve OpenID Connect Core 1.0 user claims



## auth_oidc_token

> models::IssueOidcTokenResponse auth_oidc_token(body)
Exchange authorization code or refresh token for OpenID Connect tokens

Exchanges an OAuth2 / OIDC authorization code (with PKCE code_verifier challenge verification) or refresh token for an Ed25519 JWT ID token, access token, and refresh token according to OpenID Connect Core 1.0 specifications.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::IssueOidcTokenResponse**](IssueOIDCTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_oidc_userinfo

> models::GetOidcUserInfoResponse auth_oidc_userinfo()
Retrieve OpenID Connect Core 1.0 user claims

Retrieves standard OpenID Connect profile claims (sub, email, email_verified, phone, phone_verified, name) for the authenticated caller bearing a valid Bearer access token.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetOidcUserInfoResponse**](GetOIDCUserInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

