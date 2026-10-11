# TransformApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**imageTransformGet**](TransformApi.md#imagetransformget) | **GET** /v1/image/{signature}/{path...} | Transform and optimize an image dynamically |



## imageTransformGet

> Blob imageTransformGet(signature, path)

Transform and optimize an image dynamically

Dynamically fetches, crops, resizes, optimizes, and transforms source images according to imgproxy option specifications. Verifies HMAC-SHA256 signature if enabled, caches transformed result, and streams binary image with appropriate Content-Type header.

### Example

```ts
import {
  Configuration,
  TransformApi,
} from '';
import type { ImageTransformGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TransformApi();

  const body = {
    // string
    signature: signature_example,
    // string |  (might contain slashes)
    path: path_example,
  } satisfies ImageTransformGetRequest;

  try {
    const data = await api.imageTransformGet(body);
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

### Return type

**Blob**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/octet-stream`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Transformed image binary stream |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

