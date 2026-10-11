# UserPasskeysApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserPasskeysDelete**](UserPasskeysApi.md#authuserpasskeysdelete) | **DELETE** /v1/auth/user/passkeys/{passkey_id} | Revoke a registered passkey credential |
| [**authUserPasskeysList**](UserPasskeysApi.md#authuserpasskeyslist) | **GET** /v1/auth/user/passkeys | List registered passkeys for current user |



## authUserPasskeysDelete

> any authUserPasskeysDelete(passkeyId)

Revoke a registered passkey credential

Deletes a registered WebAuthn passkey credential from auth.passkeys belonging to the authenticated user. Prohibits deleting credentials belonging to other users. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPasskeysApi,
} from '';
import type { AuthUserPasskeysDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPasskeysApi();

  const body = {
    // string
    passkeyId: passkeyId_example,
  } satisfies AuthUserPasskeysDeleteRequest;

  try {
    const data = await api.authUserPasskeysDelete(body);
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
| **passkeyId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Passkey revoked |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## authUserPasskeysList

> Array&lt;Passkey&gt; authUserPasskeysList()

List registered passkeys for current user

Retrieves all registered WebAuthn / FIDO2 passkey credentials for the authenticated user from auth.passkeys, including credential IDs, friendly names, creation timestamps, and last used dates. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPasskeysApi,
} from '';
import type { AuthUserPasskeysListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPasskeysApi();

  try {
    const data = await api.authUserPasskeysList();
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

[**Array&lt;Passkey&gt;**](Passkey.md)

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

