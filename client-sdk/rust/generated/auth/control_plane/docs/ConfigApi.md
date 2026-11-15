# \ConfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_config_get**](ConfigApi.md#auth_config_get) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration
[**auth_config_update**](ConfigApi.md#auth_config_update) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration



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

