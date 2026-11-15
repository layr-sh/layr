# \ObjectsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**file_storage_objects_delete**](ObjectsApi.md#file_storage_objects_delete) | **DELETE** /v1/file-storage/objects/{bucket}/{key...} | Permanently delete a file object
[**file_storage_objects_download**](ObjectsApi.md#file_storage_objects_download) | **GET** /v1/file-storage/objects/{bucket}/{key...} | Download a file object with byte-range support
[**file_storage_objects_head**](ObjectsApi.md#file_storage_objects_head) | **HEAD** /v1/file-storage/objects/{bucket}/{key...} | Inspect file object metadata headers
[**file_storage_objects_upload_post**](ObjectsApi.md#file_storage_objects_upload_post) | **POST** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via POST
[**file_storage_objects_upload_put**](ObjectsApi.md#file_storage_objects_upload_put) | **PUT** /v1/file-storage/objects/{bucket}/{key...} | Upload a file object via PUT



## file_storage_objects_delete

> serde_json::Value file_storage_objects_delete(bucket, key___, token, expires, op)
Permanently delete a file object

Deletes file object binary payload from the storage backend (database or S3) and removes its metadata row from file_storage.objects. Emits file_storage.object.deleted event.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**key___** | **String** |  (might contain slashes) | [required] |
**token** | Option<**String**> | Presigned capability HMAC token for deletion authorization |  |
**expires** | Option<**String**> | Unix epoch seconds timestamp when the presigned capability token expires |  |
**op** | Option<**String**> | Presigned operation capability type ('write') |  |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_objects_download

> std::path::PathBuf file_storage_objects_download(bucket, key___, token, expires, op)
Download a file object with byte-range support

Streams binary object payload from configured backend (PostgreSQL database bytea or S3) with HTTP Range header support, Content-Type, Content-Length, and ETag validation. Enforces public bucket policy or valid capability credentials.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**key___** | **String** |  (might contain slashes) | [required] |
**token** | Option<**String**> | Presigned capability HMAC token for public download |  |
**expires** | Option<**String**> | Unix epoch seconds timestamp when the presigned capability token expires |  |
**op** | Option<**String**> | Presigned operation capability type ('read') |  |

### Return type

[**std::path::PathBuf**](std::path::PathBuf.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/octet-stream

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_objects_head

> serde_json::Value file_storage_objects_head(bucket, key___, token, expires, op)
Inspect file object metadata headers

Returns HTTP metadata headers (Content-Length, Content-Type, ETag, Last-Modified) for a stored file without streaming body payload.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**key___** | **String** |  (might contain slashes) | [required] |
**token** | Option<**String**> | Presigned capability HMAC token |  |
**expires** | Option<**String**> | Unix epoch seconds timestamp when the presigned capability token expires |  |
**op** | Option<**String**> | Presigned operation capability type ('read') |  |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_objects_upload_post

> models::Object file_storage_objects_upload_post(bucket, key___, body, token, expires, op)
Upload a file object via POST

Uploads binary object payload into the target bucket, enforcing size guards and MIME validation. Stores in backend, writes metadata to file_storage.objects, and emits object created/updated event.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**key___** | **String** |  (might contain slashes) | [required] |
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |
**token** | Option<**String**> | Presigned capability HMAC token for write authorization |  |
**expires** | Option<**String**> | Unix epoch seconds timestamp when the presigned capability token expires |  |
**op** | Option<**String**> | Presigned operation capability type ('write') |  |

### Return type

[**models::Object**](Object.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_objects_upload_put

> models::Object file_storage_objects_upload_put(bucket, key___, body, token, expires, op)
Upload a file object via PUT

Stores binary payload in the specified bucket, enforcing bucket file size limits and MIME allowlists. Creates or updates metadata record in file_storage.objects and emits file_storage.object.created or file_storage.object.updated event.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**key___** | **String** |  (might contain slashes) | [required] |
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |
**token** | Option<**String**> | Presigned capability HMAC token for write authorization |  |
**expires** | Option<**String**> | Unix epoch seconds timestamp when the presigned capability token expires |  |
**op** | Option<**String**> | Presigned operation capability type ('write') |  |

### Return type

[**models::Object**](Object.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

