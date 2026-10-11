# \AnonymousApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_anonymous_sign_in**](AnonymousApi.md#auth_anonymous_sign_in) | **POST** /v1/auth/anonymous | Initialize or sign in an anonymous guest user account



## auth_anonymous_sign_in

> models::AuthTokenResponse auth_anonymous_sign_in(sign_in_anonymous_input)
Initialize or sign in an anonymous guest user account

Creates an ephemeral guest user account in auth.users with role 'authenticated', is_anonymous set to true, null email/phone, and optional initial profile properties. Publishes auth.user.signed_up event and issues an Ed25519 JWT access token, rotated refresh token in auth.sessions, and authentication cookies.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**sign_in_anonymous_input** | [**SignInAnonymousInput**](SignInAnonymousInput.md) | Request body for auth.SignInAnonymousInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

