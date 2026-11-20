# StatsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**fileStorageStatsGet**](StatsApi.md#filestoragestatsget) | **GET** /v1/_/file-storage/stats | Get file storage usage statistics |



## fileStorageStatsGet

> StatsResponse fileStorageStatsGet()

Get file storage usage statistics

Reports bucket counts, tracked object counts and logical bytes by backend, database chunk payload bytes, pending multipart usage, per-bucket usage and content type distribution. S3 usage includes only locally registered objects, not untracked upstream contents. Requires storage:bucket.read scope.

### Example

```ts
import {
  Configuration,
  StatsApi,
} from '';
import type { FileStorageStatsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new StatsApi();

  try {
    const data = await api.fileStorageStatsGet();
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

[**StatsResponse**](StatsResponse.md)

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

