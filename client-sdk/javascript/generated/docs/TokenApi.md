# TokenApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authTokenRefresh**](TokenApi.md#authtokenrefresh) | **POST** /v1/auth/token/refresh | Exchange refresh token for fresh JWT access token |



## authTokenRefresh

> AuthTokenResponse authTokenRefresh(refreshTokenInput)

Exchange refresh token for fresh JWT access token

Validates submitted refresh token against SHA-256 hash in auth.sessions, checks session expiry and user lock status, rotates refresh token, and issues a freshly signed Ed25519 JWT access token with updated claims resolved from public.auth_claims.

### Example

```ts
import {
  Configuration,
  TokenApi,
} from '';
import type { AuthTokenRefreshRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TokenApi();

  const body = {
    // RefreshTokenInput | Request body for auth.RefreshTokenInput
    refreshTokenInput: ...,
  } satisfies AuthTokenRefreshRequest;

  try {
    const data = await api.authTokenRefresh(body);
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
| **refreshTokenInput** | [RefreshTokenInput](RefreshTokenInput.md) | Request body for auth.RefreshTokenInput | |

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

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

