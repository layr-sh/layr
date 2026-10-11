# \UserPhoneVerificationApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_phone_verification_confirm**](UserPhoneVerificationApi.md#auth_user_phone_verification_confirm) | **POST** /v1/auth/user/phone/verification/confirm | Confirm phone verification code
[**auth_user_phone_verification_request**](UserPhoneVerificationApi.md#auth_user_phone_verification_request) | **POST** /v1/auth/user/phone/verification/request | Request a phone verification code for current phone



## auth_user_phone_verification_confirm

> serde_json::Value auth_user_phone_verification_confirm(confirm_phone_verification_input)
Confirm phone verification code

Verifies submitted single-use SMS code against pending verification record, updates phone_verified_at in auth.users, and marks phone as verified. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**confirm_phone_verification_input** | [**ConfirmPhoneVerificationInput**](ConfirmPhoneVerificationInput.md) | Request body for auth.ConfirmPhoneVerificationInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_phone_verification_request

> serde_json::Value auth_user_phone_verification_request(request_phone_verification_input)
Request a phone verification code for current phone

Generates and dispatches a single-use verification code via SMS to the authenticated user's registered phone number. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**request_phone_verification_input** | [**RequestPhoneVerificationInput**](RequestPhoneVerificationInput.md) | Request body for auth.RequestPhoneVerificationInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

