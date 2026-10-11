# \BucketsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**file_storage_buckets_create**](BucketsApi.md#file_storage_buckets_create) | **POST** /v1/_/file-storage/buckets | Create a storage bucket with backend configuration
[**file_storage_buckets_delete**](BucketsApi.md#file_storage_buckets_delete) | **DELETE** /v1/_/file-storage/buckets/{bucket} | Delete a storage bucket and cascade contained objects
[**file_storage_buckets_get**](BucketsApi.md#file_storage_buckets_get) | **GET** /v1/_/file-storage/buckets/{bucket} | Get storage bucket details by name
[**file_storage_buckets_list**](BucketsApi.md#file_storage_buckets_list) | **GET** /v1/_/file-storage/buckets | List all storage buckets and backend configurations
[**file_storage_buckets_list_objects**](BucketsApi.md#file_storage_buckets_list_objects) | **GET** /v1/_/file-storage/buckets/{bucket}/objects | List objects stored within a bucket
[**file_storage_buckets_update**](BucketsApi.md#file_storage_buckets_update) | **PATCH** /v1/_/file-storage/buckets/{bucket} | Update storage bucket configuration and constraints



## file_storage_buckets_create

> models::Bucket file_storage_buckets_create(create_bucket_input)
Create a storage bucket with backend configuration

Provisions a new storage bucket in file_storage.buckets with database or S3 backend settings, size limits, allowed MIME types, and public access policy. Emits file_storage.bucket.created event. Requires storage:bucket.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_bucket_input** | [**CreateBucketInput**](CreateBucketInput.md) | Request body for filestorage.CreateBucketInput | [required] |

### Return type

[**models::Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_buckets_delete

> serde_json::Value file_storage_buckets_delete(bucket)
Delete a storage bucket and cascade contained objects

Deletes a storage bucket from file_storage.buckets and permanently cascades deletion of all stored objects from the backend and database. Emits file_storage.bucket.deleted event. Requires storage:bucket.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_buckets_get

> models::Bucket file_storage_buckets_get(bucket)
Get storage bucket details by name

Retrieves configuration, size limits, allowed MIME types, backend type, and public access status for a specific bucket from file_storage.buckets. Requires storage:bucket.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |

### Return type

[**models::Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_buckets_list

> models::ListBucketsResponse file_storage_buckets_list()
List all storage buckets and backend configurations

Returns all configured storage buckets from file_storage.buckets, detailing backend type (database or s3), public access flag, file size limits, and allowed MIME types. Requires storage:bucket.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListBucketsResponse**](ListBucketsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_buckets_list_objects

> models::ListBucketObjectsResponse file_storage_buckets_list_objects(bucket)
List objects stored within a bucket

Queries metadata records in file_storage.objects for the specified bucket, detailing key paths, file sizes, MIME types, ETags, and upload timestamps with pagination. Requires storage:bucket.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |

### Return type

[**models::ListBucketObjectsResponse**](ListBucketObjectsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## file_storage_buckets_update

> models::Bucket file_storage_buckets_update(bucket, update_bucket_input)
Update storage bucket configuration and constraints

Modifies bucket settings in file_storage.buckets including public flag, file size limit, allowed MIME types, and backend configuration. Emits file_storage.bucket.updated event. Requires storage:bucket.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**bucket** | **String** |  | [required] |
**update_bucket_input** | [**UpdateBucketInput**](UpdateBucketInput.md) | Request body for filestorage.UpdateBucketInput | [required] |

### Return type

[**models::Bucket**](Bucket.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

