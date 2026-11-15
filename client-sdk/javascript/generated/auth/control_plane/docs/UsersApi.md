# UsersApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUsersCreate**](UsersApi.md#authuserscreate) | **POST** /v1/_/auth/users | Create an application user account via control plane |
| [**authUsersDelete**](UsersApi.md#authusersdelete) | **DELETE** /v1/_/auth/users/{user_id} | Permanently delete an application user and cascade sessions |
| [**authUsersGet**](UsersApi.md#authusersget) | **GET** /v1/_/auth/users/{user_id} | Get detailed application user record by UUID |
| [**authUsersList**](UsersApi.md#authuserslist) | **GET** /v1/_/auth/users | List registered application users with filtering and pagination |
| [**authUsersLock**](UsersApi.md#authuserslock) | **POST** /v1/_/auth/users/{user_id}/lock | Lock application user account and revoke active sessions |
| [**authUsersUnlock**](UsersApi.md#authusersunlock) | **DELETE** /v1/_/auth/users/{user_id}/lock | Unlock application user account and restore login access |



## authUsersCreate

> User authUsersCreate(createUserInput)

Create an application user account via control plane

Directly provisions a new user record in auth.users with specified email, normalized phone, Argon2id password hash, role, custom properties, and email/phone verification flags. Emits auth.user.created event. Requires auth:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // CreateUserInput | Request body for auth.CreateUserInput
    createUserInput: ...,
  } satisfies AuthUsersCreateRequest;

  try {
    const data = await api.authUsersCreate(body);
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
| **createUserInput** | [CreateUserInput](CreateUserInput.md) | Request body for auth.CreateUserInput | |

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


## authUsersDelete

> any authUsersDelete(userId)

Permanently delete an application user and cascade sessions

Permanently deletes application user from auth.users, cascading deletion to all active sessions, linked identities, and passkeys. Emits auth.user.deleted event. Requires auth:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies AuthUsersDeleteRequest;

  try {
    const data = await api.authUsersDelete(body);
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


## authUsersGet

> User authUsersGet(userId)

Get detailed application user record by UUID

Retrieves complete user record from auth.users by UUID, including verification timestamps, role, custom properties JSON, MFA status, lockout metadata, and linked authentication identities. Requires auth:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies AuthUsersGetRequest;

  try {
    const data = await api.authUsersGet(body);
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


## authUsersList

> ListUsersResponse authUsersList(limit, offset, role, search)

List registered application users with filtering and pagination

Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // number | Maximum number of items to return (default 50, maximum 1000) (optional)
    limit: 56,
    // number | Number of items to skip before returning results (default 0) (optional)
    offset: 56,
    // string | Filter users by role name (optional)
    role: role_example,
    // string | Case-insensitive substring search matching email or phone number (optional)
    search: search_example,
  } satisfies AuthUsersListRequest;

  try {
    const data = await api.authUsersList(body);
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
| **limit** | `number` | Maximum number of items to return (default 50, maximum 1000) | [Optional] [Defaults to `50`] |
| **offset** | `number` | Number of items to skip before returning results (default 0) | [Optional] [Defaults to `0`] |
| **role** | `string` | Filter users by role name | [Optional] [Defaults to `undefined`] |
| **search** | `string` | Case-insensitive substring search matching email or phone number | [Optional] [Defaults to `undefined`] |

### Return type

[**ListUsersResponse**](ListUsersResponse.md)

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


## authUsersLock

> User authUsersLock(userId, lockUserInput)

Lock application user account and revoke active sessions

Sets locked_until timestamp on auth.users to prevent further authentication, and immediately deletes all active sessions in auth.sessions. Emits auth.user.locked event. Requires auth:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersLockRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
    // LockUserInput | Request body for auth.LockUserInput
    lockUserInput: ...,
  } satisfies AuthUsersLockRequest;

  try {
    const data = await api.authUsersLock(body);
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
| **lockUserInput** | [LockUserInput](LockUserInput.md) | Request body for auth.LockUserInput | |

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


## authUsersUnlock

> User authUsersUnlock(userId)

Unlock application user account and restore login access

Clears locked_until timestamp in auth.users, restoring authentication capabilities for a user previously locked by control plane action or brute-force protection. Requires auth:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersApi,
} from '';
import type { AuthUsersUnlockRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies AuthUsersUnlockRequest;

  try {
    const data = await api.authUsersUnlock(body);
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

