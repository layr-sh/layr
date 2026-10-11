# UsersSessionsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUsersSessionsList**](UsersSessionsApi.md#authuserssessionslist) | **GET** /v1/_/auth/users/{user_id}/sessions | List active sessions for an application user |
| [**authUsersSessionsRevoke**](UsersSessionsApi.md#authuserssessionsrevoke) | **POST** /v1/_/auth/users/{user_id}/sessions/revoke | Revoke all active sessions for an application user |
| [**consoleUsersSessionsDelete**](UsersSessionsApi.md#consoleuserssessionsdelete) | **DELETE** /v1/_/console/users/{user_id}/sessions/{session_id} | Revoke a specific console user session |
| [**consoleUsersSessionsList**](UsersSessionsApi.md#consoleuserssessionslist) | **GET** /v1/_/console/users/{user_id}/sessions | List active sessions for a console user |
| [**consoleUsersSessionsRevoke**](UsersSessionsApi.md#consoleuserssessionsrevoke) | **POST** /v1/_/console/users/{user_id}/sessions/revoke | Revoke all active sessions for a console user |



## authUsersSessionsList

> ListUserSessionsResponse authUsersSessionsList(userId)

List active sessions for an application user

Returns all active sessions from auth.sessions for the specified user ID, detailing client IP, user agent, expiration time, and creation timestamp. Requires auth:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersSessionsApi,
} from '';
import type { AuthUsersSessionsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersSessionsApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies AuthUsersSessionsListRequest;

  try {
    const data = await api.authUsersSessionsList(body);
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

[**ListUserSessionsResponse**](ListUserSessionsResponse.md)

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


## authUsersSessionsRevoke

> any authUsersSessionsRevoke(userId, body)

Revoke all active sessions for an application user

Deletes all active sessions from auth.sessions for the specified user ID, invalidating all outstanding refresh tokens and forcing re-authentication. Requires auth:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersSessionsApi,
} from '';
import type { AuthUsersSessionsRevokeRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersSessionsApi();

  const body = {
    // string
    userId: userId_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthUsersSessionsRevokeRequest;

  try {
    const data = await api.authUsersSessionsRevoke(body);
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
| **body** | `any` | Request body for core.Empty | |

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
| **204** | Sessions revoked |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersSessionsDelete

> any consoleUsersSessionsDelete(userId, sessionId)

Revoke a specific console user session

Terminates a single session by session UUID from console.sessions for the specified console user. Emits console.user.session.revoked event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersSessionsApi,
} from '';
import type { ConsoleUsersSessionsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersSessionsApi();

  const body = {
    // string
    userId: userId_example,
    // string
    sessionId: sessionId_example,
  } satisfies ConsoleUsersSessionsDeleteRequest;

  try {
    const data = await api.consoleUsersSessionsDelete(body);
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
| **sessionId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Session revoked |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## consoleUsersSessionsList

> Array&lt;Session&gt; consoleUsersSessionsList(userId)

List active sessions for a console user

Returns all active sessions from console.sessions for the specified console user, including client IP address, user-agent string, creation timestamp, and expiration time. Requires console:user.read scope.

### Example

```ts
import {
  Configuration,
  UsersSessionsApi,
} from '';
import type { ConsoleUsersSessionsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersSessionsApi();

  const body = {
    // string
    userId: userId_example,
  } satisfies ConsoleUsersSessionsListRequest;

  try {
    const data = await api.consoleUsersSessionsList(body);
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

[**Array&lt;Session&gt;**](Session.md)

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


## consoleUsersSessionsRevoke

> any consoleUsersSessionsRevoke(userId, body)

Revoke all active sessions for a console user

Terminates all active sessions in console.sessions for the specified console user, immediately invalidating cookie tokens. Emits console.user.sessions.revoked_all event. Requires console:user.write scope.

### Example

```ts
import {
  Configuration,
  UsersSessionsApi,
} from '';
import type { ConsoleUsersSessionsRevokeRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UsersSessionsApi();

  const body = {
    // string
    userId: userId_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies ConsoleUsersSessionsRevokeRequest;

  try {
    const data = await api.consoleUsersSessionsRevoke(body);
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
| **body** | `any` | Request body for core.Empty | |

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
| **204** | Sessions revoked |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

