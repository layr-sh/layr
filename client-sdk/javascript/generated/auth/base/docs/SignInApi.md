# SignInApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authSignIn**](SignInApi.md#authsignin) | **POST** /v1/auth/sign-in | Authenticate with email or phone and password |



## authSignIn

> AuthTokenResponse authSignIn(signInInput)

Authenticate with email or phone and password

Authenticates application user with email or normalized E.164 phone and password. Verifies Argon2id hash, checks account lockout status and breach flags. If MFA is enrolled, returns an ephemeral mfa_ticket challenge; otherwise issues Ed25519 JWT access token, refresh token in auth.sessions, and session cookies.

### Example

```ts
import {
  Configuration,
  SignInApi,
} from '';
import type { AuthSignInRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SignInApi();

  const body = {
    // SignInInput | Request body for auth.SignInInput
    signInInput: ...,
  } satisfies AuthSignInRequest;

  try {
    const data = await api.authSignIn(body);
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
| **signInInput** | [SignInInput](SignInInput.md) | Request body for auth.SignInInput | |

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

