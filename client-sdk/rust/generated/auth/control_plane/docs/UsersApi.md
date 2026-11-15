# \UsersApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_users_create**](UsersApi.md#auth_users_create) | **POST** /v1/_/auth/users | Create an application user account via control plane
[**auth_users_delete**](UsersApi.md#auth_users_delete) | **DELETE** /v1/_/auth/users/{user_id} | Permanently delete an application user and cascade sessions
[**auth_users_get**](UsersApi.md#auth_users_get) | **GET** /v1/_/auth/users/{user_id} | Get detailed application user record by UUID
[**auth_users_list**](UsersApi.md#auth_users_list) | **GET** /v1/_/auth/users | List registered application users with filtering and pagination
[**auth_users_lock**](UsersApi.md#auth_users_lock) | **POST** /v1/_/auth/users/{user_id}/lock | Lock application user account and revoke active sessions
[**auth_users_unlock**](UsersApi.md#auth_users_unlock) | **DELETE** /v1/_/auth/users/{user_id}/lock | Unlock application user account and restore login access



## auth_users_create

> models::User auth_users_create(create_user_input)
Create an application user account via control plane

Directly provisions a new user record in auth.users with specified email, normalized phone, Argon2id password hash, role, custom properties, and email/phone verification flags. Emits auth.user.created event. Requires auth:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_user_input** | [**CreateUserInput**](CreateUserInput.md) | Request body for auth.CreateUserInput | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_delete

> serde_json::Value auth_users_delete(user_id)
Permanently delete an application user and cascade sessions

Permanently deletes application user from auth.users, cascading deletion to all active sessions, linked identities, and passkeys. Emits auth.user.deleted event. Requires auth:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_get

> models::User auth_users_get(user_id)
Get detailed application user record by UUID

Retrieves complete user record from auth.users by UUID, including verification timestamps, role, custom properties JSON, MFA status, lockout metadata, and linked authentication identities. Requires auth:user.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_list

> models::ListUsersResponse auth_users_list(limit, offset, role, search)
List registered application users with filtering and pagination

Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**limit** | Option<**i32**> | Maximum number of items to return (default 50, maximum 1000) |  |[default to 50]
**offset** | Option<**i32**> | Number of items to skip before returning results (default 0) |  |[default to 0]
**role** | Option<**String**> | Filter users by role name |  |
**search** | Option<**String**> | Case-insensitive substring search matching email or phone number |  |

### Return type

[**models::ListUsersResponse**](ListUsersResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_lock

> models::User auth_users_lock(user_id, lock_user_input)
Lock application user account and revoke active sessions

Sets locked_until timestamp on auth.users to prevent further authentication, and immediately deletes all active sessions in auth.sessions. Emits auth.user.locked event. Requires auth:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |
**lock_user_input** | [**LockUserInput**](LockUserInput.md) | Request body for auth.LockUserInput | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_users_unlock

> models::User auth_users_unlock(user_id)
Unlock application user account and restore login access

Clears locked_until timestamp in auth.users, restoring authentication capabilities for a user previously locked by control plane action or brute-force protection. Requires auth:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

