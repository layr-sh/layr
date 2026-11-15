# URLsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**fileStorageUrlPresign**](URLsApi.md#filestorageurlpresign) | **POST** /v1/file-storage/url/presign | Mint a presigned capability URL for temporary file access |



## fileStorageUrlPresign

> PresignURLResponse fileStorageUrlPresign(presignURLInput)

Mint a presigned capability URL for temporary file access

Generates a cryptographically signed HMAC-SHA256 capability URL granting temporary read (GET) or write (PUT) access to a specific bucket and key path with configurable expiration TTL.

### Example

```ts
import {
  Configuration,
  URLsApi,
} from '';
import type { FileStorageUrlPresignRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new URLsApi();

  const body = {
    // PresignURLInput | Request body for filestorage.PresignURLInput
    presignURLInput: ...,
  } satisfies FileStorageUrlPresignRequest;

  try {
    const data = await api.fileStorageUrlPresign(body);
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
| **presignURLInput** | [PresignURLInput](PresignURLInput.md) | Request body for filestorage.PresignURLInput | |

### Return type

[**PresignURLResponse**](PresignURLResponse.md)

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

