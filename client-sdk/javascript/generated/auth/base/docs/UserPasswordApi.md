# UserPasswordApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserPasswordUpdate**](UserPasswordApi.md#authuserpasswordupdate) | **PATCH** /v1/auth/user/password | Change or set password for current user |



## authUserPasswordUpdate

> any authUserPasswordUpdate(updateUserPasswordInput)

Change or set password for current user

Updates the account password in auth.users. Verifies current password if one is already set, enforces password complexity and breach checks, and rehashes with Argon2id. Prohibited on anonymous accounts. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPasswordApi,
} from '';
import type { AuthUserPasswordUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPasswordApi();

  const body = {
    // UpdateUserPasswordInput | Request body for auth.UpdateUserPasswordInput
    updateUserPasswordInput: ...,
  } satisfies AuthUserPasswordUpdateRequest;

  try {
    const data = await api.authUserPasswordUpdate(body);
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
| **updateUserPasswordInput** | [UpdateUserPasswordInput](UpdateUserPasswordInput.md) | Request body for auth.UpdateUserPasswordInput | |

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
| **204** | Password updated successfully |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

