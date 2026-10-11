# PasswordResetApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authPasswordResetConfirm**](PasswordResetApi.md#authpasswordresetconfirm) | **POST** /v1/auth/password-reset/confirm | Confirm password reset with verification code and new password |
| [**authPasswordResetRequest**](PasswordResetApi.md#authpasswordresetrequest) | **POST** /v1/auth/password-reset/request | Request a password reset verification code via email or SMS |



## authPasswordResetConfirm

> any authPasswordResetConfirm(confirmPasswordResetInput)

Confirm password reset with verification code and new password

Verifies single-use password recovery verification code, enforces password complexity and breach checks, updates password hash using Argon2id key derivation in auth.users, and revokes existing sessions.

### Example

```ts
import {
  Configuration,
  PasswordResetApi,
} from '';
import type { AuthPasswordResetConfirmRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasswordResetApi();

  const body = {
    // ConfirmPasswordResetInput | Request body for auth.ConfirmPasswordResetInput
    confirmPasswordResetInput: ...,
  } satisfies AuthPasswordResetConfirmRequest;

  try {
    const data = await api.authPasswordResetConfirm(body);
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
| **confirmPasswordResetInput** | [ConfirmPasswordResetInput](ConfirmPasswordResetInput.md) | Request body for auth.ConfirmPasswordResetInput | |

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
| **204** | Password reset confirmed |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authPasswordResetRequest

> any authPasswordResetRequest(requestPasswordResetInput)

Request a password reset verification code via email or SMS

Initiates password recovery for registered user, generating an Argon2id-hashed single-use verification code with cooldown guards and dispatching it to registered email or SMS destination.

### Example

```ts
import {
  Configuration,
  PasswordResetApi,
} from '';
import type { AuthPasswordResetRequestRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new PasswordResetApi();

  const body = {
    // RequestPasswordResetInput | Request body for auth.RequestPasswordResetInput
    requestPasswordResetInput: ...,
  } satisfies AuthPasswordResetRequestRequest;

  try {
    const data = await api.authPasswordResetRequest(body);
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
| **requestPasswordResetInput** | [RequestPasswordResetInput](RequestPasswordResetInput.md) | Request body for auth.RequestPasswordResetInput | |

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
| **204** | Password reset code sent |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

