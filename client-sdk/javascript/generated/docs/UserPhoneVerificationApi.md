# UserPhoneVerificationApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserPhoneVerificationConfirm**](UserPhoneVerificationApi.md#authuserphoneverificationconfirm) | **POST** /v1/auth/user/phone/verification/confirm | Confirm phone verification code |
| [**authUserPhoneVerificationRequest**](UserPhoneVerificationApi.md#authuserphoneverificationrequest) | **POST** /v1/auth/user/phone/verification/request | Request a phone verification code for current phone |



## authUserPhoneVerificationConfirm

> any authUserPhoneVerificationConfirm(confirmPhoneVerificationInput)

Confirm phone verification code

Verifies submitted single-use SMS code against pending verification record, updates phone_verified_at in auth.users, and marks phone as verified. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPhoneVerificationApi,
} from '';
import type { AuthUserPhoneVerificationConfirmRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPhoneVerificationApi();

  const body = {
    // ConfirmPhoneVerificationInput | Request body for auth.ConfirmPhoneVerificationInput
    confirmPhoneVerificationInput: ...,
  } satisfies AuthUserPhoneVerificationConfirmRequest;

  try {
    const data = await api.authUserPhoneVerificationConfirm(body);
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
| **confirmPhoneVerificationInput** | [ConfirmPhoneVerificationInput](ConfirmPhoneVerificationInput.md) | Request body for auth.ConfirmPhoneVerificationInput | |

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
| **204** | Phone successfully verified |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authUserPhoneVerificationRequest

> any authUserPhoneVerificationRequest(requestPhoneVerificationInput)

Request a phone verification code for current phone

Generates and dispatches a single-use verification code via SMS to the authenticated user\&#39;s registered phone number. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPhoneVerificationApi,
} from '';
import type { AuthUserPhoneVerificationRequestRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPhoneVerificationApi();

  const body = {
    // RequestPhoneVerificationInput | Request body for auth.RequestPhoneVerificationInput
    requestPhoneVerificationInput: ...,
  } satisfies AuthUserPhoneVerificationRequestRequest;

  try {
    const data = await api.authUserPhoneVerificationRequest(body);
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
| **requestPhoneVerificationInput** | [RequestPhoneVerificationInput](RequestPhoneVerificationInput.md) | Request body for auth.RequestPhoneVerificationInput | |

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
| **204** | Phone verification code sent |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

