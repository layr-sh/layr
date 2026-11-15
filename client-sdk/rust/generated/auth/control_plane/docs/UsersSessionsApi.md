# \UsersSessionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_users_sessions_list**](UsersSessionsApi.md#auth_users_sessions_list) | **GET** /v1/_/auth/users/{user_id}/sessions | List active sessions for an application user
[**auth_users_sessions_revoke**](UsersSessionsApi.md#auth_users_sessions_revoke) | **POST** /v1/_/auth/users/{user_id}/sessions/revoke | Revoke all active sessions for an application user



## auth_users_sessions_list

> models::ListUserSessionsResponse auth_users_sessions_list(user_id)
List active sessions for an application user

Returns all active sessions from auth.sessions for the specified user ID, detailing client IP, user agent, expiration time, and creation timestamp. Requires auth:user.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |

### Return type

[**models::ListUserSessionsResponse**](ListUserSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_sessions_revoke

> serde_json::Value auth_users_sessions_revoke(user_id, body)
Revoke all active sessions for an application user

Deletes all active sessions from auth.sessions for the specified user ID, invalidating all outstanding refresh tokens and forcing re-authentication. Requires auth:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

