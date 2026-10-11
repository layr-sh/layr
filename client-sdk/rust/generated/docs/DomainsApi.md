# \DomainsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_domains_create**](DomainsApi.md#function_domains_create) | **POST** /v1/_/function/domains | Register a standalone custom domain
[**function_domains_delete**](DomainsApi.md#function_domains_delete) | **DELETE** /v1/_/function/domains/{domain_id} | Delete custom domain and cascade attached routes
[**function_domains_list**](DomainsApi.md#function_domains_list) | **GET** /v1/_/function/domains | List registered custom domains



## function_domains_create

> models::CustomDomain function_domains_create(create_custom_domain_input)
Register a standalone custom domain

Registers a standalone custom domain hostname in function.custom_domains with verification status and TLS configuration. Emits function.domain.created event. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_custom_domain_input** | [**CreateCustomDomainInput**](CreateCustomDomainInput.md) | Request body for function.CreateCustomDomainInput | [required] |

### Return type

[**models::CustomDomain**](CustomDomain.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_domains_delete

> serde_json::Value function_domains_delete(domain_id)
Delete custom domain and cascade attached routes

Deletes a custom domain from function.custom_domains, cascading removal of all attached endpoint route mappings. Emits function.domain.deleted event. Requires function:endpoint.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**domain_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_domains_list

> models::ListCustomDomainsResponse function_domains_list()
List registered custom domains

Returns all registered custom domain definitions from function.custom_domains with hostnames and verification states. Requires function:endpoint.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**models::ListCustomDomainsResponse**](ListCustomDomainsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

