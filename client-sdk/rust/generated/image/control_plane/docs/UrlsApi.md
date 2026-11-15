# \UrlsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**image_url_sign**](UrlsApi.md#image_url_sign) | **POST** /v1/_/image/url/sign | Generate a cryptographically signed image URL



## image_url_sign

> models::SignUrlResponse image_url_sign(sign_url_input)
Generate a cryptographically signed image URL

Generates a cryptographically signed HMAC-SHA256 signature and URL path for an image transformation request using the master image key. Requires image:sign.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**sign_url_input** | [**SignUrlInput**](SignUrlInput.md) | Request body for image.SignURLInput | [required] |

### Return type

[**models::SignUrlResponse**](SignURLResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

