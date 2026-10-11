# \PasswordResetApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_password_reset_confirm**](PasswordResetApi.md#auth_password_reset_confirm) | **POST** /v1/auth/password-reset/confirm | Confirm password reset with verification code and new password
[**auth_password_reset_request**](PasswordResetApi.md#auth_password_reset_request) | **POST** /v1/auth/password-reset/request | Request a password reset verification code via email or SMS



## auth_password_reset_confirm

> serde_json::Value auth_password_reset_confirm(confirm_password_reset_input)
Confirm password reset with verification code and new password

Verifies single-use password recovery verification code, enforces password complexity and breach checks, updates password hash using Argon2id key derivation in auth.users, and revokes existing sessions.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**confirm_password_reset_input** | [**ConfirmPasswordResetInput**](ConfirmPasswordResetInput.md) | Request body for auth.ConfirmPasswordResetInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_password_reset_request

> serde_json::Value auth_password_reset_request(request_password_reset_input)
Request a password reset verification code via email or SMS

Initiates password recovery for registered user, generating an Argon2id-hashed single-use verification code with cooldown guards and dispatching it to registered email or SMS destination.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**request_password_reset_input** | [**RequestPasswordResetInput**](RequestPasswordResetInput.md) | Request body for auth.RequestPasswordResetInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

