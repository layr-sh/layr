# \SignOutApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_sign_out**](SignOutApi.md#auth_sign_out) | **POST** /v1/auth/sign-out | Revoke active session and refresh token



## auth_sign_out

> serde_json::Value auth_sign_out(body)
Revoke active session and refresh token

Revokes active refresh token, deletes corresponding session record from auth.sessions, clears session cookies, and emits auth.user.signed_out event.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

