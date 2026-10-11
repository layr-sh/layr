# ManifestApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreManifest**](ManifestApi.md#coremanifest) | **GET** /v1/manifest | Get dynamic cluster manifest and enabled services |



## coreManifest

> GetManifestResponse coreManifest()

Get dynamic cluster manifest and enabled services

Returns cluster metadata, list of enabled platform services, public server base URL, and the derived publishable key required for SDK client initialization. Unrestricted public endpoint.

### Example

```ts
import {
  Configuration,
  ManifestApi,
} from '';
import type { CoreManifestRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ManifestApi();

  try {
    const data = await api.coreManifest();
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

[**GetManifestResponse**](GetManifestResponse.md)

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

