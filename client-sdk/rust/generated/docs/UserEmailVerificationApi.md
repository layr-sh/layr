# \UserEmailVerificationApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_email_verification_confirm**](UserEmailVerificationApi.md#auth_user_email_verification_confirm) | **POST** /v1/auth/user/email/verification/confirm | Confirm email verification code
[**auth_user_email_verification_request**](UserEmailVerificationApi.md#auth_user_email_verification_request) | **POST** /v1/auth/user/email/verification/request | Request an email verification code for current email



## auth_user_email_verification_confirm

> serde_json::Value auth_user_email_verification_confirm(confirm_email_verification_input)
Confirm email verification code

Verifies submitted single-use code against pending email verification record, sets email_verified_at timestamp in auth.users, and updates primary email if part of an address change flow. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**confirm_email_verification_input** | [**ConfirmEmailVerificationInput**](ConfirmEmailVerificationInput.md) | Request body for auth.ConfirmEmailVerificationInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_email_verification_request

> serde_json::Value auth_user_email_verification_request(request_email_verification_input)
Request an email verification code for current email

Generates and dispatches a single-use verification code to the authenticated user's registered unverified email address. Rate-limited by IP and user cooldowns. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**request_email_verification_input** | [**RequestEmailVerificationInput**](RequestEmailVerificationInput.md) | Request body for auth.RequestEmailVerificationInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

