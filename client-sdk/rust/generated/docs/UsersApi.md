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
[**console_users_create**](UsersApi.md#console_users_create) | **POST** /v1/_/console/users | Create console user account
[**console_users_delete**](UsersApi.md#console_users_delete) | **DELETE** /v1/_/console/users/{user_id} | Delete console user account and cascade sessions
[**console_users_get**](UsersApi.md#console_users_get) | **GET** /v1/_/console/users/{user_id} | Get console user account details by UUID
[**console_users_list**](UsersApi.md#console_users_list) | **GET** /v1/_/console/users | List console user accounts
[**console_users_reset_password**](UsersApi.md#console_users_reset_password) | **POST** /v1/_/console/users/{user_id}/password | Reset console user password and revoke sessions
[**console_users_update**](UsersApi.md#console_users_update) | **PUT** /v1/_/console/users/{user_id} | Update console user account properties



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

> models::ListUsersResponse auth_users_list()
List registered application users with filtering and pagination

Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope.

### Parameters

This endpoint does not need any parameter.

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


## console_users_create

> models::User console_users_create(create_console_user_input)
Create console user account

Creates a new console user account in console.users with email and Argon2id password hash (minimum 8 characters). Publishes console.user.created event. Requires console:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_console_user_input** | [**CreateConsoleUserInput**](CreateConsoleUserInput.md) | Request body for console.CreateConsoleUserInput | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_users_delete

> serde_json::Value console_users_delete(user_id)
Delete console user account and cascade sessions

Permanently deletes console user account from console.users, automatically cascading revocation of all active sessions in console.sessions. Emits console.user.deleted event. Requires console:user.write scope.

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


## console_users_get

> models::User console_users_get(user_id)
Get console user account details by UUID

Retrieves a single console user account from console.users by UUID, including email, enabled status, MFA enrollment flag, and timestamps. Requires console:user.read scope.

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


## console_users_list

> Vec<models::User> console_users_list()
List console user accounts

Returns all registered console user accounts from console.users with email, active status, MFA enrollment status, last login timestamp, and creation date. Requires console:user.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**Vec<models::User>**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_users_reset_password

> serde_json::Value console_users_reset_password(user_id, reset_console_user_password_input)
Reset console user password and revoke sessions

Updates password hash in console.users using Argon2id key derivation and revokes all active sessions in console.sessions, forcing re-login across all devices. Emits console.user.password_reset event. Requires console:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |
**reset_console_user_password_input** | [**ResetConsoleUserPasswordInput**](ResetConsoleUserPasswordInput.md) | Request body for console.ResetConsoleUserPasswordInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_users_update

> models::User console_users_update(user_id, update_console_user_input)
Update console user account properties

Modifies email or enabled status of a console user in console.users. If account is disabled, immediately revokes all active sessions in console.sessions. Emits console.user.updated event. Requires console:user.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**user_id** | **String** |  | [required] |
**update_console_user_input** | [**UpdateConsoleUserInput**](UpdateConsoleUserInput.md) | Request body for console.UpdateConsoleUserInput | [required] |

### Return type

[**models::User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

