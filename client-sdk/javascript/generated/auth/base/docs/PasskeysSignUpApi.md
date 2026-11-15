# PasskeysSignUpApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authPasskeysSignUpBegin**](PasskeysSignUpApi.md#authpasskeyssignupbegin) | **POST** /v1/auth/passkeys/sign-up | Begin WebAuthn passkey registration ceremony |
| [**authPasskeysSignUpVerify**](PasskeysSignUpApi.md#authpasskeyssignupverify) | **POST** /v1/auth/passkeys/sign-up/verify | Verify WebAuthn passkey registration attestation and store credential |



## authPasskeysSignUpBegin

> SignUpOptions authPasskeysSignUpBegin(beginPasskeySignUpInput)

Begin WebAuthn passkey registration ceremony

Initiates WebAuthn / FIDO2 credential creation ceremony, generating an attestation challenge, relying party metadata, and user entity options for hardware or platform passkey enrollment.

### Example

```ts
import {
  Configuration,
  PasskeysSignUpApi,
} from '';
import type { AuthPasskeysSignUpBeginRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasskeysSignUpApi();

  const body = {
    // BeginPasskeySignUpInput | Request body for auth.BeginPasskeySignUpInput
    beginPasskeySignUpInput: ...,
  } satisfies AuthPasskeysSignUpBeginRequest;

  try {
    const data = await api.authPasskeysSignUpBegin(body);
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
| **beginPasskeySignUpInput** | [BeginPasskeySignUpInput](BeginPasskeySignUpInput.md) | Request body for auth.BeginPasskeySignUpInput | |

### Return type

[**SignUpOptions**](SignUpOptions.md)

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


## authPasskeysSignUpVerify

> AuthTokenResponse authPasskeysSignUpVerify(verifyPasskeySignUpInput)

Verify WebAuthn passkey registration attestation and store credential

Validates client attestation response and public key, stores credential record in auth.passkeys, creates user identity, and issues an authenticated session with Ed25519 JWT tokens.

### Example

```ts
import {
  Configuration,
  PasskeysSignUpApi,
} from '';
import type { AuthPasskeysSignUpVerifyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasskeysSignUpApi();

  const body = {
    // VerifyPasskeySignUpInput | Request body for auth.VerifyPasskeySignUpInput
    verifyPasskeySignUpInput: ...,
  } satisfies AuthPasskeysSignUpVerifyRequest;

  try {
    const data = await api.authPasskeysSignUpVerify(body);
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
| **verifyPasskeySignUpInput** | [VerifyPasskeySignUpInput](VerifyPasskeySignUpInput.md) | Request body for auth.VerifyPasskeySignUpInput | |

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

