# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_config_get**](ConfigApi.md#auth_config_get) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration
[**auth_config_update**](ConfigApi.md#auth_config_update) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration
[**console_config_get**](ConfigApi.md#console_config_get) | **GET** /v1/_/console/config | Get dynamic runtime console configuration
[**console_config_update**](ConfigApi.md#console_config_update) | **PUT** /v1/_/console/config | Update dynamic runtime console configuration
[**data_config_get**](ConfigApi.md#data_config_get) | **GET** /v1/_/data/config | Retrieve dynamic data service configuration
[**data_config_update**](ConfigApi.md#data_config_update) | **PUT** /v1/_/data/config | Update dynamic data service configuration
[**file_storage_config_get**](ConfigApi.md#file_storage_config_get) | **GET** /v1/_/file-storage/config | Get runtime file storage configuration
[**file_storage_config_update**](ConfigApi.md#file_storage_config_update) | **PUT** /v1/_/file-storage/config | Update runtime file storage configuration
[**function_config_get**](ConfigApi.md#function_config_get) | **GET** /v1/_/function/config | Get runtime function configuration
[**function_config_update**](ConfigApi.md#function_config_update) | **PUT** /v1/_/function/config | Update runtime function configuration
[**image_config_get**](ConfigApi.md#image_config_get) | **GET** /v1/_/image/config | Get runtime image service configuration
[**image_config_update**](ConfigApi.md#image_config_update) | **PUT** /v1/_/image/config | Update runtime image service configuration
[**tasks_config_get**](ConfigApi.md#tasks_config_get) | **GET** /v1/_/tasks/config | Get runtime tasks orchestrator configuration
[**tasks_config_update**](ConfigApi.md#tasks_config_update) | **PUT** /v1/_/tasks/config | Update runtime tasks orchestrator configuration



## auth_config_get

> models::Config auth_config_get()
Retrieve dynamic runtime authentication configuration

Returns active dynamic authentication settings from auth.config including password strength rules, session TTLs, rate limits, MFA policies, and enabled OAuth providers, with client secrets projected as boolean indicators. Requires auth:config.read scope.

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


## auth_config_update

> models::Config auth_config_update(config)
Update dynamic runtime authentication configuration

Updates dynamic authentication configuration parameters in auth.config without requiring server restarts. Automatically envelope-encrypts newly supplied third-party OAuth provider secrets with AES-256-GCM using the master encryption key. Requires auth:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for auth.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## console_config_get

> models::Config console_config_get()
Get dynamic runtime console configuration

Returns active runtime console configuration settings including session duration, MFA enforcement policy, and allowed origins. Requires console:config.read scope.

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


## console_config_update

> models::Config console_config_update(config)
Update dynamic runtime console configuration

Mutates runtime console configuration parameters in console.config, validating structure and applying settings dynamically across cluster nodes. Requires console:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for console.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_config_get

> models::Config data_config_get()
Retrieve dynamic data service configuration

Returns dynamic data service configuration settings including exposed schema names, connection pool sizes, REST limit defaults, and cache TTL parameters. Requires data:config.read scope.

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


## data_config_update

> models::Config data_config_update(config)
Update dynamic data service configuration

Updates operational data service parameters in data.config, dynamically tuning connection limits, exposed schemas, and cache behaviors without server downtime. Requires data:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for data.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


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


## function_config_get

> models::Config function_config_get()
Get runtime function configuration

Returns operational function runtime settings including default isolate timeouts, memory limits, runner concurrency, and isolate pool capacities. Requires function:config.read scope.

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


## function_config_update

> models::Config function_config_update(config)
Update runtime function configuration

Modifies operational function runtime settings in function.config, tuning concurrency limits, memory caps, and execution timeouts without restarting the gateway. Requires function:config.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**config** | [**Config**](Config.md) | Request body for function.Config | [required] |

### Return type

[**models::Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


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

