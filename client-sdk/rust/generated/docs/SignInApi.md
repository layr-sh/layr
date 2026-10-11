# \SignInApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_sign_in**](SignInApi.md#auth_sign_in) | **POST** /v1/auth/sign-in | Authenticate with email or phone and password



## auth_sign_in

> models::AuthTokenResponse auth_sign_in(sign_in_input)
Authenticate with email or phone and password

Authenticates application user with email or normalized E.164 phone and password. Verifies Argon2id hash, checks account lockout status and breach flags. If MFA is enrolled, returns an ephemeral mfa_ticket challenge; otherwise issues Ed25519 JWT access token, refresh token in auth.sessions, and session cookies.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**sign_in_input** | [**SignInInput**](SignInInput.md) | Request body for auth.SignInInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

