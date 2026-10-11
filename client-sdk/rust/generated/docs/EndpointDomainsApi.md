# \EndpointDomainsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_endpoint_domains_create**](EndpointDomainsApi.md#function_endpoint_domains_create) | **POST** /v1/_/function/endpoints/{endpoint_id}/domains | Attach endpoint route to custom domain with path prefix
[**function_endpoint_domains_delete**](EndpointDomainsApi.md#function_endpoint_domains_delete) | **DELETE** /v1/_/function/endpoints/{endpoint_id}/domains/{route_id} | Remove endpoint route mapping from custom domain
[**function_endpoint_domains_list**](EndpointDomainsApi.md#function_endpoint_domains_list) | **GET** /v1/_/function/endpoints/{endpoint_id}/domains | List custom domain routes attached to an endpoint



## function_endpoint_domains_create

> models::CustomDomainRoute function_endpoint_domains_create(endpoint_id, create_custom_domain_route_input)
Attach endpoint route to custom domain with path prefix

Maps a custom domain hostname and URL path prefix to a specific serverless endpoint in function.custom_domain_routes, enabling custom domain routing. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**create_custom_domain_route_input** | [**CreateCustomDomainRouteInput**](CreateCustomDomainRouteInput.md) | Request body for function.CreateCustomDomainRouteInput | [required] |

### Return type

[**models::CustomDomainRoute**](CustomDomainRoute.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoint_domains_delete

> serde_json::Value function_endpoint_domains_delete(endpoint_id, route_id)
Remove endpoint route mapping from custom domain

Removes a specific domain route mapping from function.custom_domain_routes, unbinding the custom path prefix from the endpoint. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**route_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_endpoint_domains_list

> models::ListCustomDomainRoutesResponse function_endpoint_domains_list(endpoint_id)
List custom domain routes attached to an endpoint

Returns all custom domain route mappings from function.custom_domain_routes configured for a specific serverless endpoint. Requires function:endpoint.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |

### Return type

[**models::ListCustomDomainRoutesResponse**](ListCustomDomainRoutesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

