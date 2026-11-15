# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**file_storage_config_get**](ConfigApi.md#file_storage_config_get) | **GET** /v1/_/file-storage/config | Get runtime file storage configuration
[**file_storage_config_update**](ConfigApi.md#file_storage_config_update) | **PUT** /v1/_/file-storage/config | Update runtime file storage configuration



## file_storage_config_get

> models::Config file_storage_config_get()
Get runtime file storage configuration

Returns dynamic operational configuration settings for the file storage subsystem including global size limits and default backends. Requires storage:config.read scope.

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


## file_storage_config_update

> models::Config file_storage_config_update(config)
Update runtime file storage configuration

Updates dynamic operational configuration for the file storage subsystem in storage.config, tuning global upload limits and backends without restart. Requires storage:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for filestorage.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

