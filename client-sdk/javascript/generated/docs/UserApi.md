# UserApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserDelete**](UserApi.md#authuserdelete) | **DELETE** /v1/auth/user | Permanently delete authenticated user account |
| [**authUserExport**](UserApi.md#authuserexport) | **POST** /v1/auth/user/export | Export comprehensive user account data graph (GDPR Article 20) |
| [**authUserGet**](UserApi.md#authuserget) | **GET** /v1/auth/user | Get profile and verification details of authenticated user |



## authUserDelete

> any authUserDelete()

Permanently delete authenticated user account

Permanently deletes the authenticated user from auth.users, cascading deletion across auth.sessions, auth.identities, auth.passkeys, and user properties. Clears authentication cookies and emits auth.user.deleted event. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserApi,
} from '';
import type { AuthUserDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserApi();

  try {
    const data = await api.authUserDelete();
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
| **204** | Account deleted successfully |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authUserExport

> ExportUserResponse authUserExport(body)

Export comprehensive user account data graph (GDPR Article 20)

Exports full personal data graph associated with the authenticated user, including profile metadata, linked authentication identities, active sessions, and passkey registrations in portable JSON format. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserApi,
} from '';
import type { AuthUserExportRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthUserExportRequest;

  try {
    const data = await api.authUserExport(body);
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

[**ExportUserResponse**](ExportUserResponse.md)

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


## authUserGet

> GetUserResponse authUserGet()

Get profile and verification details of authenticated user

Retrieves authenticated user profile from auth.users, including email, phone, verification timestamps, role, custom properties JSON, MFA enrollment status, and lockout flags. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserApi,
} from '';
import type { AuthUserGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserApi();

  try {
    const data = await api.authUserGet();
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

[**GetUserResponse**](GetUserResponse.md)

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

