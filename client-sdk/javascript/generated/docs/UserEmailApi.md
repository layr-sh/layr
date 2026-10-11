# UserEmailApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserEmailUpdate**](UserEmailApi.md#authuseremailupdate) | **PATCH** /v1/auth/user/email | Request update of authenticated user email address |



## authUserEmailUpdate

> any authUserEmailUpdate(updateUserEmailInput)

Request update of authenticated user email address

Asserts uniqueness of the new email address, generates an Argon2id-hashed single-use verification code, and dispatches it to the requested email. The address is updated once verified. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserEmailApi,
} from '';
import type { AuthUserEmailUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserEmailApi();

  const body = {
    // UpdateUserEmailInput | Request body for auth.UpdateUserEmailInput
    updateUserEmailInput: ...,
  } satisfies AuthUserEmailUpdateRequest;

  try {
    const data = await api.authUserEmailUpdate(body);
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
| **updateUserEmailInput** | [UpdateUserEmailInput](UpdateUserEmailInput.md) | Request body for auth.UpdateUserEmailInput | |

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

