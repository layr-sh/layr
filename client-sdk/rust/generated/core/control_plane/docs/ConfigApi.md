# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_config_get**](ConfigApi.md#core_config_get) | **GET** /v1/_/core/config | Get Core runtime configuration
[**core_config_update**](ConfigApi.md#core_config_update) | **PUT** /v1/_/core/config | Update Core runtime configuration



## core_config_get

> models::RuntimeConfig core_config_get()
Get Core runtime configuration

#### Controller:   `layr.sh/core.(*Server).handleGetConfig`  ---  

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::RuntimeConfig**](RuntimeConfig.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_config_update

> models::RuntimeConfig core_config_update(runtime_config)
Update Core runtime configuration

#### Controller:   `layr.sh/core.(*Server).handleUpdateConfig`  ---  

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**runtime_config** | [**RuntimeConfig**](RuntimeConfig.md) | Request body for core.RuntimeConfig | [required] |

### Return type

[**models::RuntimeConfig**](RuntimeConfig.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

