# SignUpApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authSignUp**](SignUpApi.md#authsignup) | **POST** /v1/auth/sign-up | Register a new application user with password |



## authSignUp

> AuthTokenResponse authSignUp(signUpInput)

Register a new application user with password

Registers a new application user account with email or normalized E.164 phone, password complexity validation, breach detection, and Argon2id key derivation. Supports seamless upgrade of anonymous guest accounts. Dispatches verification code if confirmation is required; otherwise issues Ed25519 JWT tokens.

### Example

```ts
import {
  Configuration,
  SignUpApi,
} from '';
import type { AuthSignUpRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SignUpApi();

  const body = {
    // SignUpInput | Request body for auth.SignUpInput
    signUpInput: ...,
  } satisfies AuthSignUpRequest;

  try {
    const data = await api.authSignUp(body);
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
| **signUpInput** | [SignUpInput](SignUpInput.md) | Request body for auth.SignUpInput | |

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

