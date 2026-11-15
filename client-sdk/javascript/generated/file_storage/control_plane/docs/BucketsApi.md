# BucketsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**fileStorageBucketsCreate**](BucketsApi.md#filestoragebucketscreate) | **POST** /v1/_/file-storage/buckets | Create a storage bucket with backend configuration |
| [**fileStorageBucketsDelete**](BucketsApi.md#filestoragebucketsdelete) | **DELETE** /v1/_/file-storage/buckets/{bucket} | Delete a storage bucket and cascade contained objects |
| [**fileStorageBucketsGet**](BucketsApi.md#filestoragebucketsget) | **GET** /v1/_/file-storage/buckets/{bucket} | Get storage bucket details by name |
| [**fileStorageBucketsList**](BucketsApi.md#filestoragebucketslist) | **GET** /v1/_/file-storage/buckets | List all storage buckets and backend configurations |
| [**fileStorageBucketsListObjects**](BucketsApi.md#filestoragebucketslistobjects) | **GET** /v1/_/file-storage/buckets/{bucket}/objects | List objects stored within a bucket |
| [**fileStorageBucketsUpdate**](BucketsApi.md#filestoragebucketsupdate) | **PATCH** /v1/_/file-storage/buckets/{bucket} | Update storage bucket configuration and constraints |



## fileStorageBucketsCreate

> Bucket fileStorageBucketsCreate(createBucketInput)

Create a storage bucket with backend configuration

Provisions a new storage bucket in file_storage.buckets with database or S3 backend settings, size limits, allowed MIME types, and public access policy. Emits file_storage.bucket.created event. Requires storage:bucket.write scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  const body = {
    // CreateBucketInput | Request body for filestorage.CreateBucketInput
    createBucketInput: ...,
  } satisfies FileStorageBucketsCreateRequest;

  try {
    const data = await api.fileStorageBucketsCreate(body);
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
| **createBucketInput** | [CreateBucketInput](CreateBucketInput.md) | Request body for filestorage.CreateBucketInput | |

### Return type

[**Bucket**](Bucket.md)

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


## fileStorageBucketsDelete

> any fileStorageBucketsDelete(bucket)

Delete a storage bucket and cascade contained objects

Deletes a storage bucket from file_storage.buckets and permanently cascades deletion of all stored objects from the backend and database. Emits file_storage.bucket.deleted event. Requires storage:bucket.write scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  const body = {
    // string
    bucket: bucket_example,
  } satisfies FileStorageBucketsDeleteRequest;

  try {
    const data = await api.fileStorageBucketsDelete(body);
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
| **204** | Bucket deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## fileStorageBucketsGet

> Bucket fileStorageBucketsGet(bucket)

Get storage bucket details by name

Retrieves configuration, size limits, allowed MIME types, backend type, and public access status for a specific bucket from file_storage.buckets. Requires storage:bucket.read scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  const body = {
    // string
    bucket: bucket_example,
  } satisfies FileStorageBucketsGetRequest;

  try {
    const data = await api.fileStorageBucketsGet(body);
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

### Return type

[**Bucket**](Bucket.md)

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


## fileStorageBucketsList

> ListBucketsResponse fileStorageBucketsList()

List all storage buckets and backend configurations

Returns all configured storage buckets from file_storage.buckets, detailing backend type (database or s3), public access flag, file size limits, and allowed MIME types. Requires storage:bucket.read scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  try {
    const data = await api.fileStorageBucketsList();
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

[**ListBucketsResponse**](ListBucketsResponse.md)

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


## fileStorageBucketsListObjects

> ListBucketObjectsResponse fileStorageBucketsListObjects(bucket, limit, offset)

List objects stored within a bucket

Queries metadata records in file_storage.objects for the specified bucket, detailing key paths, file sizes, MIME types, ETags, and upload timestamps with pagination. Requires storage:bucket.read scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsListObjectsRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  const body = {
    // string
    bucket: bucket_example,
    // number | Maximum number of items to return (default 50, maximum 1000) (optional)
    limit: 56,
    // number | Number of items to skip before returning results (default 0) (optional)
    offset: 56,
  } satisfies FileStorageBucketsListObjectsRequest;

  try {
    const data = await api.fileStorageBucketsListObjects(body);
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
| **limit** | `number` | Maximum number of items to return (default 50, maximum 1000) | [Optional] [Defaults to `50`] |
| **offset** | `number` | Number of items to skip before returning results (default 0) | [Optional] [Defaults to `0`] |

### Return type

[**ListBucketObjectsResponse**](ListBucketObjectsResponse.md)

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


## fileStorageBucketsUpdate

> Bucket fileStorageBucketsUpdate(bucket, updateBucketInput)

Update storage bucket configuration and constraints

Modifies bucket settings in file_storage.buckets including public flag, file size limit, allowed MIME types, and backend configuration. Emits file_storage.bucket.updated event. Requires storage:bucket.write scope.

### Example

```ts
import {
  Configuration,
  BucketsApi,
} from '';
import type { FileStorageBucketsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BucketsApi();

  const body = {
    // string
    bucket: bucket_example,
    // UpdateBucketInput | Request body for filestorage.UpdateBucketInput
    updateBucketInput: ...,
  } satisfies FileStorageBucketsUpdateRequest;

  try {
    const data = await api.fileStorageBucketsUpdate(body);
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
| **updateBucketInput** | [UpdateBucketInput](UpdateBucketInput.md) | Request body for filestorage.UpdateBucketInput | |

### Return type

[**Bucket**](Bucket.md)

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

