# AnonymousApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authAnonymousSignIn**](AnonymousApi.md#authanonymoussignin) | **POST** /v1/auth/anonymous | Initialize or sign in an anonymous guest user account |



## authAnonymousSignIn

> AuthTokenResponse authAnonymousSignIn(signInAnonymousInput)

Initialize or sign in an anonymous guest user account

Creates an ephemeral guest user account in auth.users with role \&#39;authenticated\&#39;, is_anonymous set to true, null email/phone, and optional initial profile properties. Publishes auth.user.signed_up event and issues an Ed25519 JWT access token, rotated refresh token in auth.sessions, and authentication cookies.

### Example

```ts
import {
  Configuration,
  AnonymousApi,
} from '';
import type { AuthAnonymousSignInRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new AnonymousApi();

  const body = {
    // SignInAnonymousInput | Request body for auth.SignInAnonymousInput
    signInAnonymousInput: ...,
  } satisfies AuthAnonymousSignInRequest;

  try {
    const data = await api.authAnonymousSignIn(body);
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
| **signInAnonymousInput** | [SignInAnonymousInput](SignInAnonymousInput.md) | Request body for auth.SignInAnonymousInput | |

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

