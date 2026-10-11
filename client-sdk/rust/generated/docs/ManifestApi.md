# \ManifestApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_manifest**](ManifestApi.md#core_manifest) | **GET** /v1/manifest | Get dynamic cluster manifest and enabled services



## core_manifest

> models::GetManifestResponse core_manifest()
Get dynamic cluster manifest and enabled services

Returns cluster metadata, list of enabled platform services, public server base URL, and the derived publishable key required for SDK client initialization. Unrestricted public endpoint.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::GetManifestResponse**](GetManifestResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

