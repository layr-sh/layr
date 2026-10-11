# InfoApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**imageInfoGet**](InfoApi.md#imageinfoget) | **GET** /v1/image/info/{signature}/{path...} | Inspect image metadata by path |
| [**imageInfoPost**](InfoApi.md#imageinfopost) | **POST** /v1/image/info | Inspect image metadata from binary upload |



## imageInfoGet

> GetInfoResponse imageInfoGet(signature, path, token, expires, op)

Inspect image metadata by path

Extracts structural metadata (width, height, format, file size, colorspace, EXIF, orientation) from a remote or storage image path using cryptographic signature verification.

### Example

```ts
import {
  Configuration,
  InfoApi,
} from '';
import type { ImageInfoGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new InfoApi();

  const body = {
    // string
    signature: signature_example,
    // string |  (might contain slashes)
    path: path_example,
    // string | Presigned capability HMAC token for protected storage sources (optional)
    token: token_example,
    // string | Unix epoch seconds expiration timestamp for presigned capability (optional)
    expires: expires_example,
    // string | Presigned operation capability type (\'read\') (optional)
    op: op_example,
  } satisfies ImageInfoGetRequest;

  try {
    const data = await api.imageInfoGet(body);
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
| **signature** | `string` |  | [Defaults to `undefined`] |
| **path** | `string` |  (might contain slashes) | [Defaults to `undefined`] |
| **token** | `string` | Presigned capability HMAC token for protected storage sources | [Optional] [Defaults to `undefined`] |
| **expires** | `string` | Unix epoch seconds expiration timestamp for presigned capability | [Optional] [Defaults to `undefined`] |
| **op** | `string` | Presigned operation capability type (\&#39;read\&#39;) | [Optional] [Defaults to `undefined`] |

### Return type

[**GetInfoResponse**](GetInfoResponse.md)

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


## imageInfoPost

> GetInfoResponse imageInfoPost(body)

Inspect image metadata from binary upload

Probes and extracts dimensions, format, file size, colorspace, and EXIF metadata from an uploaded image binary payload.

### Example

```ts
import {
  Configuration,
  InfoApi,
} from '';
import type { ImageInfoPostRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new InfoApi();

  const body = {
    // any | Request body for core.Empty
    body: ...,
  } satisfies ImageInfoPostRequest;

  try {
    const data = await api.imageInfoPost(body);
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

[**GetInfoResponse**](GetInfoResponse.md)

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

