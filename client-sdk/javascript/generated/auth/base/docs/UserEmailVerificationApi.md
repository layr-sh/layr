# UserEmailVerificationApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserEmailVerificationConfirm**](UserEmailVerificationApi.md#authuseremailverificationconfirm) | **POST** /v1/auth/user/email/verification/confirm | Confirm email verification code |
| [**authUserEmailVerificationRequest**](UserEmailVerificationApi.md#authuseremailverificationrequest) | **POST** /v1/auth/user/email/verification/request | Request an email verification code for current email |



## authUserEmailVerificationConfirm

> any authUserEmailVerificationConfirm(confirmEmailVerificationInput)

Confirm email verification code

Verifies submitted single-use code against pending email verification record, sets email_verified_at timestamp in auth.users, and updates primary email if part of an address change flow. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserEmailVerificationApi,
} from '';
import type { AuthUserEmailVerificationConfirmRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserEmailVerificationApi();

  const body = {
    // ConfirmEmailVerificationInput | Request body for auth.ConfirmEmailVerificationInput
    confirmEmailVerificationInput: ...,
  } satisfies AuthUserEmailVerificationConfirmRequest;

  try {
    const data = await api.authUserEmailVerificationConfirm(body);
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
| **confirmEmailVerificationInput** | [ConfirmEmailVerificationInput](ConfirmEmailVerificationInput.md) | Request body for auth.ConfirmEmailVerificationInput | |

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
| **204** | Email successfully verified |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authUserEmailVerificationRequest

> any authUserEmailVerificationRequest(requestEmailVerificationInput)

Request an email verification code for current email

Generates and dispatches a single-use verification code to the authenticated user\&#39;s registered unverified email address. Rate-limited by IP and user cooldowns. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserEmailVerificationApi,
} from '';
import type { AuthUserEmailVerificationRequestRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserEmailVerificationApi();

  const body = {
    // RequestEmailVerificationInput | Request body for auth.RequestEmailVerificationInput
    requestEmailVerificationInput: ...,
  } satisfies AuthUserEmailVerificationRequestRequest;

  try {
    const data = await api.authUserEmailVerificationRequest(body);
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
| **requestEmailVerificationInput** | [RequestEmailVerificationInput](RequestEmailVerificationInput.md) | Request body for auth.RequestEmailVerificationInput | |

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
| **204** | Email verification code sent |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

