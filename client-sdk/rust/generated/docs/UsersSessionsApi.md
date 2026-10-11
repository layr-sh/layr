# \UsersSessionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_users_sessions_list**](UsersSessionsApi.md#auth_users_sessions_list) | **GET** /v1/_/auth/users/{user_id}/sessions | List active sessions for an application user
[**auth_users_sessions_revoke**](UsersSessionsApi.md#auth_users_sessions_revoke) | **POST** /v1/_/auth/users/{user_id}/sessions/revoke | Revoke all active sessions for an application user
[**console_users_sessions_delete**](UsersSessionsApi.md#console_users_sessions_delete) | **DELETE** /v1/_/console/users/{user_id}/sessions/{session_id} | Revoke a specific console user session
[**console_users_sessions_list**](UsersSessionsApi.md#console_users_sessions_list) | **GET** /v1/_/console/users/{user_id}/sessions | List active sessions for a console user
[**console_users_sessions_revoke**](UsersSessionsApi.md#console_users_sessions_revoke) | **POST** /v1/_/console/users/{user_id}/sessions/revoke | Revoke all active sessions for a console user



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


## console_users_sessions_delete

> serde_json::Value console_users_sessions_delete(user_id, session_id)
Revoke a specific console user session

Terminates a single session by session UUID from console.sessions for the specified console user. Emits console.user.session.revoked event. Requires console:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |
**session_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_users_sessions_list

> Vec<models::Session> console_users_sessions_list(user_id)
List active sessions for a console user

Returns all active sessions from console.sessions for the specified console user, including client IP address, user-agent string, creation timestamp, and expiration time. Requires console:user.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |

### Return type

[**Vec<models::Session>**](Session.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_users_sessions_revoke

> serde_json::Value console_users_sessions_revoke(user_id, body)
Revoke all active sessions for a console user

Terminates all active sessions in console.sessions for the specified console user, immediately invalidating cookie tokens. Emits console.user.sessions.revoked_all event. Requires console:user.write scope.

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

