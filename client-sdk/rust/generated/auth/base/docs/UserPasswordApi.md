# \UserPasswordApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_password_update**](UserPasswordApi.md#auth_user_password_update) | **PATCH** /v1/auth/user/password | Change or set password for current user



## auth_user_password_update

> serde_json::Value auth_user_password_update(update_user_password_input)
Change or set password for current user

Updates the account password in auth.users. Verifies current password if one is already set, enforces password complexity and breach checks, and rehashes with Argon2id. Prohibited on anonymous accounts. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**update_user_password_input** | [**UpdateUserPasswordInput**](UpdateUserPasswordInput.md) | Request body for auth.UpdateUserPasswordInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

