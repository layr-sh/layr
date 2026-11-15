# \UserPhoneApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_phone_update**](UserPhoneApi.md#auth_user_phone_update) | **PATCH** /v1/auth/user/phone | Request update of authenticated user phone number



## auth_user_phone_update

> serde_json::Value auth_user_phone_update(update_user_phone_input)
Request update of authenticated user phone number

Normalizes new phone number to E.164, verifies uniqueness in auth.users, generates single-use verification code, and dispatches via SMS. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**update_user_phone_input** | [**UpdateUserPhoneInput**](UpdateUserPhoneInput.md) | Request body for auth.UpdateUserPhoneInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

