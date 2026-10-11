# \TransformApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**image_transform_get**](TransformApi.md#image_transform_get) | **GET** /v1/image/{signature}/{path...} | Transform and optimize an image dynamically



## image_transform_get

> std::path::PathBuf image_transform_get(signature, path___, token, expires, op)
Transform and optimize an image dynamically

Dynamically fetches, crops, resizes, optimizes, and transforms source images according to imgproxy option specifications. Verifies HMAC-SHA256 signature if enabled, caches transformed result, and streams binary image with appropriate Content-Type header.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**signature** | **String** |  | [required] |
**path___** | **String** |  (might contain slashes) | [required] |
**token** | Option<**String**> | Presigned capability HMAC token for protected storage sources |  |
**expires** | Option<**String**> | Unix epoch seconds expiration timestamp for presigned capability |  |
**op** | Option<**String**> | Presigned operation capability type ('read') |  |

### Return type

[**std::path::PathBuf**](std::path::PathBuf.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/octet-stream

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

