# MFAApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authMfaChallenge**](MFAApi.md#authmfachallenge) | **POST** /v1/auth/mfa/challenge | Verify MFA challenge ticket with TOTP passcode |
| [**authMfaDisable**](MFAApi.md#authmfadisable) | **DELETE** /v1/auth/mfa | Disable multi-factor authentication on user account |
| [**authMfaSetup**](MFAApi.md#authmfasetup) | **POST** /v1/auth/mfa | Generate TOTP secret and setup URI for authenticator enrollment |
| [**authMfaVerify**](MFAApi.md#authmfaverify) | **POST** /v1/auth/mfa/verify | Verify initial TOTP code to complete MFA enrollment |



## authMfaChallenge

> AuthTokenResponse authMfaChallenge(challengeMFAInput)

Verify MFA challenge ticket with TOTP passcode

Verifies a 6-digit TOTP passcode against the user\&#39;s enrolled MFA secret using a short-lived challenge ticket (5-minute TTL) issued during password sign-in. On verification, issues an Ed25519 JWT access token, rotated refresh token, and creates a session record in auth.sessions.

### Example

```ts
import {
  Configuration,
  MFAApi,
} from '';
import type { AuthMfaChallengeRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new MFAApi();

  const body = {
    // ChallengeMFAInput | Request body for auth.ChallengeMFAInput
    challengeMFAInput: ...,
  } satisfies AuthMfaChallengeRequest;

  try {
    const data = await api.authMfaChallenge(body);
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
| **challengeMFAInput** | [ChallengeMFAInput](ChallengeMFAInput.md) | Request body for auth.ChallengeMFAInput | |

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


## authMfaDisable

> any authMfaDisable()

Disable multi-factor authentication on user account

Disables TOTP multi-factor authentication on auth.users for the authenticated user, clears the encrypted MFA secret, and publishes auth.mfa.disabled event. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  MFAApi,
} from '';
import type { AuthMfaDisableRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new MFAApi();

  try {
    const data = await api.authMfaDisable();
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
| **204** | MFA disabled on account |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authMfaSetup

> SetupMFAResponse authMfaSetup(body)

Generate TOTP secret and setup URI for authenticator enrollment

Generates a cryptographic RFC 6238 TOTP secret, stores it temporarily encrypted in the session cache, and returns an otpauth:// URI and QR code payload compatible with Google Authenticator, 1Password, or Authy. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  MFAApi,
} from '';
import type { AuthMfaSetupRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new MFAApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthMfaSetupRequest;

  try {
    const data = await api.authMfaSetup(body);
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

[**SetupMFAResponse**](SetupMFAResponse.md)

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


## authMfaVerify

> AuthTokenResponse authMfaVerify(verifyMFAInput)

Verify initial TOTP code to complete MFA enrollment

Validates the initial 6-digit TOTP code against the pending setup secret, sets mfa_enabled to true on auth.users, stores the AES-encrypted secret permanently, and publishes auth.mfa.enabled event. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  MFAApi,
} from '';
import type { AuthMfaVerifyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new MFAApi();

  const body = {
    // VerifyMFAInput | Request body for auth.VerifyMFAInput
    verifyMFAInput: ...,
  } satisfies AuthMfaVerifyRequest;

  try {
    const data = await api.authMfaVerify(body);
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
| **verifyMFAInput** | [VerifyMFAInput](VerifyMFAInput.md) | Request body for auth.VerifyMFAInput | |

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

