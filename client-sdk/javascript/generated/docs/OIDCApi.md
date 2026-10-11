# OIDCApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authOidcToken**](OIDCApi.md#authoidctoken) | **POST** /v1/auth/oauth/token | Exchange authorization code or refresh token for OpenID Connect tokens |
| [**authOidcUserinfo**](OIDCApi.md#authoidcuserinfo) | **GET** /v1/auth/oauth/userinfo | Retrieve OpenID Connect Core 1.0 user claims |



## authOidcToken

> IssueOIDCTokenResponse authOidcToken(body)

Exchange authorization code or refresh token for OpenID Connect tokens

Exchanges an OAuth2 / OIDC authorization code (with PKCE code_verifier challenge verification) or refresh token for an Ed25519 JWT ID token, access token, and refresh token according to OpenID Connect Core 1.0 specifications.

### Example

```ts
import {
  Configuration,
  OIDCApi,
} from '';
import type { AuthOidcTokenRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new OIDCApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies AuthOidcTokenRequest;

  try {
    const data = await api.authOidcToken(body);
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

[**IssueOIDCTokenResponse**](IssueOIDCTokenResponse.md)

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


## authOidcUserinfo

> GetOIDCUserInfoResponse authOidcUserinfo()

Retrieve OpenID Connect Core 1.0 user claims

Retrieves standard OpenID Connect profile claims (sub, email, email_verified, phone, phone_verified, name) for the authenticated caller bearing a valid Bearer access token.

### Example

```ts
import {
  Configuration,
  OIDCApi,
} from '';
import type { AuthOidcUserinfoRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new OIDCApi();

  try {
    const data = await api.authOidcUserinfo();
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

[**GetOIDCUserInfoResponse**](GetOIDCUserInfoResponse.md)

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

