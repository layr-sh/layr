# UsersSessionsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUsersSessionsList**](UsersSessionsApi.md#authuserssessionslist) | **GET** /v1/_/auth/users/{user_id}/sessions | List active sessions for an application user |
| [**authUsersSessionsRevoke**](UsersSessionsApi.md#authuserssessionsrevoke) | **POST** /v1/_/auth/users/{user_id}/sessions/revoke | Revoke all active sessions for an application user |



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

