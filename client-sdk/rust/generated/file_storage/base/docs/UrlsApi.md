# \UrlsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**file_storage_url_presign**](UrlsApi.md#file_storage_url_presign) | **POST** /v1/file-storage/url/presign | Mint a presigned capability URL for temporary file access



## file_storage_url_presign

> models::PresignUrlResponse file_storage_url_presign(presign_url_input)
Mint a presigned capability URL for temporary file access

Generates a cryptographically signed HMAC-SHA256 capability URL granting temporary read (GET) or write (PUT) access to a specific bucket and key path with configurable expiration TTL.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**presign_url_input** | [**PresignUrlInput**](PresignUrlInput.md) | Request body for filestorage.PresignURLInput | [required] |

### Return type

[**models::PresignUrlResponse**](PresignURLResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

