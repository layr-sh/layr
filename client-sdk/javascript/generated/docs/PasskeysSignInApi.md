# PasskeysSignInApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authPasskeysSignInBegin**](PasskeysSignInApi.md#authpasskeyssigninbegin) | **POST** /v1/auth/passkeys/sign-in | Begin WebAuthn passkey authentication ceremony |
| [**authPasskeysSignInVerify**](PasskeysSignInApi.md#authpasskeyssigninverify) | **POST** /v1/auth/passkeys/sign-in/verify | Verify WebAuthn passkey assertion signature and issue session |



## authPasskeysSignInBegin

> SignInOptions authPasskeysSignInBegin(body)

Begin WebAuthn passkey authentication ceremony

Initiates WebAuthn / FIDO2 assertion ceremony, generating a cryptographic challenge and credential request options for the client browser or authenticator hardware.

### Example

```ts
import {
  Configuration,
  PasskeysSignInApi,
} from '';
import type { AuthPasskeysSignInBeginRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasskeysSignInApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthPasskeysSignInBeginRequest;

  try {
    const data = await api.authPasskeysSignInBegin(body);
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

[**SignInOptions**](SignInOptions.md)

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


## authPasskeysSignInVerify

> AuthTokenResponse authPasskeysSignInVerify(verifyPasskeySignInInput)

Verify WebAuthn passkey assertion signature and issue session

Verifies the client\&#39;s WebAuthn assertion signature against the registered public key and signature counter stored in auth.passkeys. Issues an Ed25519 JWT access token, refresh token, and session record in auth.sessions.

### Example

```ts
import {
  Configuration,
  PasskeysSignInApi,
} from '';
import type { AuthPasskeysSignInVerifyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasskeysSignInApi();

  const body = {
    // VerifyPasskeySignInInput | Request body for auth.VerifyPasskeySignInInput
    verifyPasskeySignInInput: ...,
  } satisfies AuthPasskeysSignInVerifyRequest;

  try {
    const data = await api.authPasskeysSignInVerify(body);
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
| **verifyPasskeySignInInput** | [VerifyPasskeySignInInput](VerifyPasskeySignInInput.md) | Request body for auth.VerifyPasskeySignInInput | |

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

