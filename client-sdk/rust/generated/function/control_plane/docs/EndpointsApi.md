# \EndpointsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_endpoints_create**](EndpointsApi.md#function_endpoints_create) | **POST** /v1/_/function/endpoints | Register a new serverless endpoint
[**function_endpoints_delete**](EndpointsApi.md#function_endpoints_delete) | **DELETE** /v1/_/function/endpoints/{endpoint_id} | Delete serverless endpoint and unload runtime isolates
[**function_endpoints_executions**](EndpointsApi.md#function_endpoints_executions) | **GET** /v1/_/function/endpoints/{endpoint_id}/executions | Query execution history logs for a specific endpoint
[**function_endpoints_get**](EndpointsApi.md#function_endpoints_get) | **GET** /v1/_/function/endpoints/{endpoint_id} | Get serverless endpoint definition by ID
[**function_endpoints_list**](EndpointsApi.md#function_endpoints_list) | **GET** /v1/_/function/endpoints | List all registered serverless endpoints
[**function_endpoints_update**](EndpointsApi.md#function_endpoints_update) | **PUT** /v1/_/function/endpoints/{endpoint_id} | Update serverless endpoint definition



## function_endpoints_create

> models::Endpoint function_endpoints_create(create_endpoint_input)
Register a new serverless endpoint

Provisions a new serverless endpoint definition in function.endpoints with route path, name, runtime environment (Node, Deno, Python, Go), timeout guards, memory limits, and environment variables. Emits function.endpoint.created event. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_endpoint_input** | [**CreateEndpointInput**](CreateEndpointInput.md) | Request body for function.CreateEndpointInput | [required] |

### Return type

[**models::Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoints_delete

> serde_json::Value function_endpoints_delete(endpoint_id)
Delete serverless endpoint and unload runtime isolates

Permanently deletes an endpoint from function.endpoints, unloads running worker isolates, cascades historical deployments, and removes custom domain routes. Emits function.endpoint.deleted event. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoints_executions

> models::ListExecutionsResponse function_endpoints_executions(endpoint_id, limit, offset)
Query execution history logs for a specific endpoint

Returns historical invocation audit records from function.executions for a specific endpoint, detailing response codes, durations, memory usage, and execution log streams. Requires function:endpoint.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**limit** | Option<**i32**> | Maximum number of items to return (default 50, maximum 1000) |  |[default to 50]
**offset** | Option<**i32**> | Number of items to skip before returning results (default 0) |  |[default to 0]

### Return type

[**models::ListExecutionsResponse**](ListExecutionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoints_get

> models::Endpoint function_endpoints_get(endpoint_id)
Get serverless endpoint definition by ID

Retrieves complete definition of a serverless endpoint from function.endpoints by ID or name, including runtime configuration, environment variables, and active deployment version. Requires function:endpoint.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |

### Return type

[**models::Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoints_list

> models::ListEndpointsResponse function_endpoints_list()
List all registered serverless endpoints

Returns all registered serverless endpoints from function.endpoints with route names, runtime engines, timeout guards, memory limits, active deployment IDs, and status. Requires function:endpoint.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListEndpointsResponse**](ListEndpointsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoints_update

> models::Endpoint function_endpoints_update(endpoint_id, update_endpoint_input)
Update serverless endpoint definition

Modifies an existing serverless endpoint definition in function.endpoints, updating environment variables, timeout guards, or memory limits. Emits function.endpoint.updated event. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**update_endpoint_input** | [**UpdateEndpointInput**](UpdateEndpointInput.md) | Request body for function.UpdateEndpointInput | [required] |

### Return type

[**models::Endpoint**](Endpoint.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

