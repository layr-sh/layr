# \UserEmailApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_email_update**](UserEmailApi.md#auth_user_email_update) | **PATCH** /v1/auth/user/email | Request update of authenticated user email address



## auth_user_email_update

> serde_json::Value auth_user_email_update(update_user_email_input)
Request update of authenticated user email address

Asserts uniqueness of the new email address, generates an Argon2id-hashed single-use verification code, and dispatches it to the requested email. The address is updated once verified. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**update_user_email_input** | [**UpdateUserEmailInput**](UpdateUserEmailInput.md) | Request body for auth.UpdateUserEmailInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

