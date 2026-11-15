# \UserSessionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_sessions_list**](UserSessionsApi.md#auth_user_sessions_list) | **GET** /v1/auth/user/sessions | List active sessions for current user
[**auth_user_sessions_revoke**](UserSessionsApi.md#auth_user_sessions_revoke) | **DELETE** /v1/auth/user/sessions/{session_id} | Revoke a specific user session by ID
[**auth_user_sessions_revoke_others**](UserSessionsApi.md#auth_user_sessions_revoke_others) | **POST** /v1/auth/user/sessions/revoke-others | Sign out of all other devices and sessions



## auth_user_sessions_list

> models::ListSessionsResponse auth_user_sessions_list()
List active sessions for current user

Returns active sessions from auth.sessions for the authenticated user, detailing IP address, user agent, creation date, expiry timestamp, and a flag indicating the current device session. Requires authenticated user session.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListSessionsResponse**](ListSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_sessions_revoke

> serde_json::Value auth_user_sessions_revoke(session_id)
Revoke a specific user session by ID

Deletes a specific session record from auth.sessions belonging to the authenticated user, terminating access for that device. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**session_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_sessions_revoke_others

> models::RevokeOtherSessionsResponse auth_user_sessions_revoke_others(body)
Sign out of all other devices and sessions

Revokes all active sessions in auth.sessions belonging to the authenticated user except the current device session, invalidating other refresh tokens immediately. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::RevokeOtherSessionsResponse**](RevokeOtherSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

