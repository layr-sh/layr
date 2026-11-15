# \UserApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_delete**](UserApi.md#auth_user_delete) | **DELETE** /v1/auth/user | Permanently delete authenticated user account
[**auth_user_export**](UserApi.md#auth_user_export) | **POST** /v1/auth/user/export | Export comprehensive user account data graph (GDPR Article 20)
[**auth_user_get**](UserApi.md#auth_user_get) | **GET** /v1/auth/user | Get profile and verification details of authenticated user



## auth_user_delete

> serde_json::Value auth_user_delete()
Permanently delete authenticated user account

Permanently deletes the authenticated user from auth.users, cascading deletion across auth.sessions, auth.identities, auth.passkeys, and user properties. Clears authentication cookies and emits auth.user.deleted event. Requires authenticated user session.

### Parameters

This endpoint does not need any parameter.

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_export

> models::ExportUserResponse auth_user_export(body)
Export comprehensive user account data graph (GDPR Article 20)

Exports full personal data graph associated with the authenticated user, including profile metadata, linked authentication identities, active sessions, and passkey registrations in portable JSON format. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::ExportUserResponse**](ExportUserResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_get

> models::GetUserResponse auth_user_get()
Get profile and verification details of authenticated user

Retrieves authenticated user profile from auth.users, including email, phone, verification timestamps, role, custom properties JSON, MFA enrollment status, and lockout flags. Requires authenticated user session.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetUserResponse**](GetUserResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

