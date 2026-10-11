# \InfoApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**image_info_get**](InfoApi.md#image_info_get) | **GET** /v1/image/info/{signature}/{path...} | Inspect image metadata by path
[**image_info_post**](InfoApi.md#image_info_post) | **POST** /v1/image/info | Inspect image metadata from binary upload



## image_info_get

> models::GetInfoResponse image_info_get(signature, path___)
Inspect image metadata by path

Extracts structural metadata (width, height, format, file size, colorspace, EXIF, orientation) from a remote or storage image path using cryptographic signature verification.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**signature** | **String** |  | [required] |
**path___** | **String** |  (might contain slashes) | [required] |

### Return type

[**models::GetInfoResponse**](GetInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_info_post

> models::GetInfoResponse image_info_post(body)
Inspect image metadata from binary upload

Probes and extracts dimensions, format, file size, colorspace, and EXIF metadata from an uploaded image binary payload.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::GetInfoResponse**](GetInfoResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

