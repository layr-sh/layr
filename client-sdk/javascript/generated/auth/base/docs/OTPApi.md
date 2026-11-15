# OTPApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authOtpSend**](OTPApi.md#authotpsend) | **POST** /v1/auth/otp | Request a one-time password verification code via email or SMS |
| [**authOtpVerify**](OTPApi.md#authotpverify) | **POST** /v1/auth/otp/verify | Verify one-time password code and issue authenticated session |



## authOtpSend

> any authOtpSend(sendOTPInput)

Request a one-time password verification code via email or SMS

Generates a single-use 6-digit numeric OTP with configurable TTL and rate limiting, hashes it with Argon2id, stores it in auth.otps, and dispatches it via configured email or SMS provider (Twilio, Vonage, SendGrid, Resend, or SMTP).

### Example

```ts
import {
  Configuration,
  OTPApi,
} from '';
import type { AuthOtpSendRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new OTPApi();

  const body = {
    // SendOTPInput | Request body for auth.SendOTPInput
    sendOTPInput: ...,
  } satisfies AuthOtpSendRequest;

  try {
    const data = await api.authOtpSend(body);
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
| **sendOTPInput** | [SendOTPInput](SendOTPInput.md) | Request body for auth.SendOTPInput | |

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
| **204** | OTP verification code dispatched |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authOtpVerify

> AuthTokenResponse authOtpVerify(verifyOTPInput)

Verify one-time password code and issue authenticated session

Validates the 6-digit OTP code against stored Argon2id hash in auth.otps. On successful verification, marks contact method verified, issues an Ed25519 JWT access token and rotated refresh token, and creates an active session in auth.sessions.

### Example

```ts
import {
  Configuration,
  OTPApi,
} from '';
import type { AuthOtpVerifyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new OTPApi();

  const body = {
    // VerifyOTPInput | Request body for auth.VerifyOTPInput
    verifyOTPInput: ...,
  } satisfies AuthOtpVerifyRequest;

  try {
    const data = await api.authOtpVerify(body);
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
| **verifyOTPInput** | [VerifyOTPInput](VerifyOTPInput.md) | Request body for auth.VerifyOTPInput | |

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

