# UserSessionsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserSessionsList**](UserSessionsApi.md#authusersessionslist) | **GET** /v1/auth/user/sessions | List active sessions for current user |
| [**authUserSessionsRevoke**](UserSessionsApi.md#authusersessionsrevoke) | **DELETE** /v1/auth/user/sessions/{session_id} | Revoke a specific user session by ID |
| [**authUserSessionsRevokeOthers**](UserSessionsApi.md#authusersessionsrevokeothers) | **POST** /v1/auth/user/sessions/revoke-others | Sign out of all other devices and sessions |



## authUserSessionsList

> ListSessionsResponse authUserSessionsList()

List active sessions for current user

Returns active sessions from auth.sessions for the authenticated user, detailing IP address, user agent, creation date, expiry timestamp, and a flag indicating the current device session. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserSessionsApi,
} from '';
import type { AuthUserSessionsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserSessionsApi();

  try {
    const data = await api.authUserSessionsList();
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

[**ListSessionsResponse**](ListSessionsResponse.md)

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


## authUserSessionsRevoke

> any authUserSessionsRevoke(sessionId)

Revoke a specific user session by ID

Deletes a specific session record from auth.sessions belonging to the authenticated user, terminating access for that device. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserSessionsApi,
} from '';
import type { AuthUserSessionsRevokeRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserSessionsApi();

  const body = {
    // string
    sessionId: sessionId_example,
  } satisfies AuthUserSessionsRevokeRequest;

  try {
    const data = await api.authUserSessionsRevoke(body);
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


## authUserSessionsRevokeOthers

> RevokeOtherSessionsResponse authUserSessionsRevokeOthers(body)

Sign out of all other devices and sessions

Revokes all active sessions in auth.sessions belonging to the authenticated user except the current device session, invalidating other refresh tokens immediately. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserSessionsApi,
} from '';
import type { AuthUserSessionsRevokeOthersRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserSessionsApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthUserSessionsRevokeOthersRequest;

  try {
    const data = await api.authUserSessionsRevokeOthers(body);
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
| **body** | `any` | Request body for core.Empty | |

### Return type

[**RevokeOtherSessionsResponse**](RevokeOtherSessionsResponse.md)

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

