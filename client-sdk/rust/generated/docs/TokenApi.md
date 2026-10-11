# \TokenApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_token_refresh**](TokenApi.md#auth_token_refresh) | **POST** /v1/auth/token/refresh | Exchange refresh token for fresh JWT access token



## auth_token_refresh

> models::AuthTokenResponse auth_token_refresh(refresh_token_input)
Exchange refresh token for fresh JWT access token

Validates submitted refresh token against SHA-256 hash in auth.sessions, checks session expiry and user lock status, rotates refresh token, and issues a freshly signed Ed25519 JWT access token with updated claims resolved from public.auth_claims.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**refresh_token_input** | [**RefreshTokenInput**](RefreshTokenInput.md) | Request body for auth.RefreshTokenInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

