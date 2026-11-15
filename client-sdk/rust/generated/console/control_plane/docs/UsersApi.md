# \UsersApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**console_users_create**](UsersApi.md#console_users_create) | **POST** /v1/_/console/users | Create console user account
[**console_users_delete**](UsersApi.md#console_users_delete) | **DELETE** /v1/_/console/users/{user_id} | Delete console user account and cascade sessions
[**console_users_get**](UsersApi.md#console_users_get) | **GET** /v1/_/console/users/{user_id} | Get console user account details by UUID
[**console_users_list**](UsersApi.md#console_users_list) | **GET** /v1/_/console/users | List console user accounts
[**console_users_reset_password**](UsersApi.md#console_users_reset_password) | **POST** /v1/_/console/users/{user_id}/password | Reset console user password and revoke sessions
[**console_users_update**](UsersApi.md#console_users_update) | **PUT** /v1/_/console/users/{user_id} | Update console user account properties



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

Returns all registered console user accounts from console.users with email, active status, MFA enrollment status, last signed in timestamp, and creation date. Requires console:user.read scope.

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

