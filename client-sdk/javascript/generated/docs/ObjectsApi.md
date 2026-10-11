# ObjectsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**fileStorageObjectsDelete**](ObjectsApi.md#filestorageobjectsdelete) | **DELETE** /v1/file-storage/objects/{bucket}/{key...} | Permanently delete a file object |
| [**fileStorageObjectsDownload**](ObjectsApi.md#filestorageobjectsdownload) | **GET** /v1/file-storage/objects/{bucket}/{key...} | Download a file object with byte-range support |
| [**fileStorageObjectsHead**](ObjectsApi.md#filestorageobjectshead) | **HEAD** /v1/file-storage/objects/{bucket}/{key...} | Inspect file object metadata headers |
| [**fileStorageObjectsUploadPost**](ObjectsApi.md#filestorageobjectsuploadpost) | **POST** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via POST |
| [**fileStorageObjectsUploadPut**](ObjectsApi.md#filestorageobjectsuploadput) | **PUT** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via PUT |



## fileStorageObjectsDelete

> any fileStorageObjectsDelete(bucket, key)

Permanently delete a file object

Deletes file object binary payload from the storage backend (database or S3) and removes its metadata row from file_storage.objects. Emits file_storage.object.deleted event.

### Example

```ts
import {
  Configuration,
  ObjectsApi,
} from '';
import type { FileStorageObjectsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ObjectsApi();

  const body = {
    // string
    bucket: bucket_example,
    // string |  (might contain slashes)
    key: key_example,
  } satisfies FileStorageObjectsDeleteRequest;

  try {
    const data = await api.fileStorageObjectsDelete(body);
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
| **bucket** | `string` |  | [Defaults to `undefined`] |
| **key** | `string` |  (might contain slashes) | [Defaults to `undefined`] |

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
| **204** | Object deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## fileStorageObjectsDownload

> Blob fileStorageObjectsDownload(bucket, key)

Download a file object with byte-range support

Streams binary object payload from configured backend (PostgreSQL database bytea or S3) with HTTP Range header support, Content-Type, Content-Length, and ETag validation. Enforces public bucket policy or valid capability credentials.

### Example

```ts
import {
  Configuration,
  ObjectsApi,
} from '';
import type { FileStorageObjectsDownloadRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ObjectsApi();

  const body = {
    // string
    bucket: bucket_example,
    // string |  (might contain slashes)
    key: key_example,
  } satisfies FileStorageObjectsDownloadRequest;

  try {
    const data = await api.fileStorageObjectsDownload(body);
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
| **bucket** | `string` |  | [Defaults to `undefined`] |
| **key** | `string` |  (might contain slashes) | [Defaults to `undefined`] |

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
| **200** | File binary stream |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## fileStorageObjectsHead

> any fileStorageObjectsHead(bucket, key)

Inspect file object metadata headers

Returns HTTP metadata headers (Content-Length, Content-Type, ETag, Last-Modified) for a stored file without streaming body payload.

### Example

```ts
import {
  Configuration,
  ObjectsApi,
} from '';
import type { FileStorageObjectsHeadRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ObjectsApi();

  const body = {
    // string
    bucket: bucket_example,
    // string |  (might contain slashes)
    key: key_example,
  } satisfies FileStorageObjectsHeadRequest;

  try {
    const data = await api.fileStorageObjectsHead(body);
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
| **bucket** | `string` |  | [Defaults to `undefined`] |
| **key** | `string` |  (might contain slashes) | [Defaults to `undefined`] |

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

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## fileStorageObjectsUploadPost

> ModelObject fileStorageObjectsUploadPost(bucket, key, body)

Upload a file object via POST

Uploads binary object payload into the target bucket, enforcing size guards and MIME validation. Stores in backend, writes metadata to file_storage.objects, and emits object created/updated event.

### Example

```ts
import {
  Configuration,
  ObjectsApi,
} from '';
import type { FileStorageObjectsUploadPostRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ObjectsApi();

  const body = {
    // string
    bucket: bucket_example,
    // string |  (might contain slashes)
    key: key_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies FileStorageObjectsUploadPostRequest;

  try {
    const data = await api.fileStorageObjectsUploadPost(body);
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
| **bucket** | `string` |  | [Defaults to `undefined`] |
| **key** | `string` |  (might contain slashes) | [Defaults to `undefined`] |
| **body** | `any` | Request body for core.Empty | |

### Return type

[**ModelObject**](ModelObject.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## fileStorageObjectsUploadPut

> ModelObject fileStorageObjectsUploadPut(bucket, key, body)

Upload a file object via PUT

Stores binary payload in the specified bucket, enforcing bucket file size limits and MIME allowlists. Creates or updates metadata record in file_storage.objects and emits file_storage.object.created or file_storage.object.updated event.

### Example

```ts
import {
  Configuration,
  ObjectsApi,
} from '';
import type { FileStorageObjectsUploadPutRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ObjectsApi();

  const body = {
    // string
    bucket: bucket_example,
    // string |  (might contain slashes)
    key: key_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies FileStorageObjectsUploadPutRequest;

  try {
    const data = await api.fileStorageObjectsUploadPut(body);
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
| **bucket** | `string` |  | [Defaults to `undefined`] |
| **key** | `string` |  (might contain slashes) | [Defaults to `undefined`] |
| **body** | `any` | Request body for core.Empty | |

### Return type

[**ModelObject**](ModelObject.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

