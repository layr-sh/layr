# UsersApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**consoleUsersCreate**](UsersApi.md#consoleuserscreate) | **POST** /v1/_/console/users | Create console user account |
| [**consoleUsersDelete**](UsersApi.md#consoleusersdelete) | **DELETE** /v1/_/console/users/{user_id} | Delete console user account and cascade sessions |
| [**consoleUsersGet**](UsersApi.md#consoleusersget) | **GET** /v1/_/console/users/{user_id} | Get console user account details by UUID |
| [**consoleUsersList**](UsersApi.md#consoleuserslist) | **GET** /v1/_/console/users | List console user accounts |
| [**consoleUsersResetPassword**](UsersApi.md#consoleusersresetpassword) | **POST** /v1/_/console/users/{user_id}/password | Reset console user password and revoke sessions |
| [**consoleUsersUpdate**](UsersApi.md#consoleusersupdate) | **PUT** /v1/_/console/users/{user_id} | Update console user account properties |



## consoleUsersCreate

> User consoleUsersCreate(createConsoleUserInput)

Create console user account

Creates a new console user account in console.users with email and Argon2id password hash (minimum 8 characters). Publishes console.user.created event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // CreateConsoleUserInput | Request body for console.CreateConsoleUserInput
    createConsoleUserInput: ...,
  } satisfies ConsoleUsersCreateRequest;

  try {
    const data = await api.consoleUsersCreate(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **createConsoleUserInput** | [CreateConsoleUserInput](CreateConsoleUserInput.md) | Request body for console.CreateConsoleUserInput | |

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersDelete

> any consoleUsersDelete(userId)

Delete console user account and cascade sessions

Permanently deletes console user account from console.users, automatically cascading revocation of all active sessions in console.sessions. Emits console.user.deleted event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies ConsoleUsersDeleteRequest;

  try {
    const data = await api.consoleUsersDelete(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **userId** | `string` |  | [Defaults to `undefined`] |

### Return type

**any**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **204** | User deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersGet

> User consoleUsersGet(userId)

Get console user account details by UUID

Retrieves a single console user account from console.users by UUID, including email, enabled status, MFA enrollment flag, and timestamps. Requires console:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies ConsoleUsersGetRequest;

  try {
    const data = await api.consoleUsersGet(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **userId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersList

> Array&lt;User&gt; consoleUsersList()

List console user accounts

Returns all registered console user accounts from console.users with email, active status, MFA enrollment status, last signed in timestamp, and creation date. Requires console:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  try {
    const data = await api.consoleUsersList();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**Array&lt;User&gt;**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersResetPassword

> any consoleUsersResetPassword(userId, resetConsoleUserPasswordInput)

Reset console user password and revoke sessions

Updates password hash in console.users using Argon2id key derivation and revokes all active sessions in console.sessions, forcing re-login across all devices. Emits console.user.password_reset event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersResetPasswordRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
    // ResetConsoleUserPasswordInput | Request body for console.ResetConsoleUserPasswordInput
    resetConsoleUserPasswordInput: ...,
  } satisfies ConsoleUsersResetPasswordRequest;

  try {
    const data = await api.consoleUsersResetPassword(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **userId** | `string` |  | [Defaults to `undefined`] |
| **resetConsoleUserPasswordInput** | [ResetConsoleUserPasswordInput](ResetConsoleUserPasswordInput.md) | Request body for console.ResetConsoleUserPasswordInput | |

### Return type

**any**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersUpdate

> User consoleUsersUpdate(userId, updateConsoleUserInput)

Update console user account properties

Modifies email or enabled status of a console user in console.users. If account is disabled, immediately revokes all active sessions in console.sessions. Emits console.user.updated event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { ConsoleUsersUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
    // UpdateConsoleUserInput | Request body for console.UpdateConsoleUserInput
    updateConsoleUserInput: ...,
  } satisfies ConsoleUsersUpdateRequest;

  try {
    const data = await api.consoleUsersUpdate(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **userId** | `string` |  | [Defaults to `undefined`] |
| **updateConsoleUserInput** | [UpdateConsoleUserInput](UpdateConsoleUserInput.md) | Request body for console.UpdateConsoleUserInput | |

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

