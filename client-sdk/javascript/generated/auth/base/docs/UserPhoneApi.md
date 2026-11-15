# UserPhoneApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserPhoneUpdate**](UserPhoneApi.md#authuserphoneupdate) | **PATCH** /v1/auth/user/phone | Request update of authenticated user phone number |



## authUserPhoneUpdate

> any authUserPhoneUpdate(updateUserPhoneInput)

Request update of authenticated user phone number

Normalizes new phone number to E.164, verifies uniqueness in auth.users, generates single-use verification code, and dispatches via SMS. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPhoneApi,
} from '';
import type { AuthUserPhoneUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPhoneApi();

  const body = {
    // UpdateUserPhoneInput | Request body for auth.UpdateUserPhoneInput
    updateUserPhoneInput: ...,
  } satisfies AuthUserPhoneUpdateRequest;

  try {
    const data = await api.authUserPhoneUpdate(body);
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
| **updateUserPhoneInput** | [UpdateUserPhoneInput](UpdateUserPhoneInput.md) | Request body for auth.UpdateUserPhoneInput | |

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

