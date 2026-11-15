# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**image_config_get**](ConfigApi.md#image_config_get) | **GET** /v1/_/image/config | Get runtime image service configuration
[**image_config_update**](ConfigApi.md#image_config_update) | **PUT** /v1/_/image/config | Update runtime image service configuration



## image_config_get

> models::Config image_config_get()
Get runtime image service configuration

Returns operational image service settings including compression quality defaults, maximum resolution limits, allowed source domains, and signature enforcement policy. Requires image:config.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## image_config_update

> models::Config image_config_update(config)
Update runtime image service configuration

Modifies operational image service parameters in image.config, updating compression quality, resolution limits, and domain allowlists without restarting the service. Requires image:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for image.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

