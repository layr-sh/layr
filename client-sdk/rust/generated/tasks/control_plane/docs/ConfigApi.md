# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**tasks_config_get**](ConfigApi.md#tasks_config_get) | **GET** /v1/_/tasks/config | Get runtime tasks orchestrator configuration
[**tasks_config_update**](ConfigApi.md#tasks_config_update) | **PUT** /v1/_/tasks/config | Update runtime tasks orchestrator configuration



## tasks_config_get

> models::Config tasks_config_get()
Get runtime tasks orchestrator configuration

Returns operational task configuration settings including worker concurrency limits, poller interval, default timeout guards, and exponential retry backoff parameters. Requires tasks:config.read scope.

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


## tasks_config_update

> models::Config tasks_config_update(config)
Update runtime tasks orchestrator configuration

Updates operational task configuration in tasks.config, dynamically adjusting worker concurrency, polling intervals, and retry backoff limits without restarting workers. Requires tasks:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for tasks.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

