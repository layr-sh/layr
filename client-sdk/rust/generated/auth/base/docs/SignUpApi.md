# \SignUpApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_sign_up**](SignUpApi.md#auth_sign_up) | **POST** /v1/auth/sign-up | Register a new application user with password



## auth_sign_up

> models::AuthTokenResponse auth_sign_up(sign_up_input)
Register a new application user with password

Registers a new application user account with email or normalized E.164 phone, password complexity validation, breach detection, and Argon2id key derivation. Supports seamless upgrade of anonymous guest accounts. Dispatches verification code if confirmation is required; otherwise issues Ed25519 JWT tokens.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**sign_up_input** | [**SignUpInput**](SignUpInput.md) | Request body for auth.SignUpInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

