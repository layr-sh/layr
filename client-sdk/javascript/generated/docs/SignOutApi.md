# SignOutApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authSignOut**](SignOutApi.md#authsignout) | **POST** /v1/auth/sign-out | Revoke active session and refresh token |



## authSignOut

> any authSignOut(body)

Revoke active session and refresh token

Revokes active refresh token, deletes corresponding session record from auth.sessions, clears session cookies, and emits auth.user.signed_out event.

### Example

```ts
import {
  Configuration,
  SignOutApi,
} from '';
import type { AuthSignOutRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SignOutApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthSignOutRequest;

  try {
    const data = await api.authSignOut(body);
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
| **204** | Signed out successfully |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

