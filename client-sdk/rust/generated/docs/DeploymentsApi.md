# \DeploymentsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**function_deployments_create**](DeploymentsApi.md#function_deployments_create) | **POST** /v1/_/function/endpoints/{endpoint_id}/deploy | Create immutable deployment and hot-reload runner
[**function_deployments_list**](DeploymentsApi.md#function_deployments_list) | **GET** /v1/_/function/endpoints/{endpoint_id}/deployments | List immutable deployments for an endpoint
[**function_deployments_rollback**](DeploymentsApi.md#function_deployments_rollback) | **POST** /v1/_/function/endpoints/{endpoint_id}/rollback | Rollback endpoint to a previous deployment revision



## function_deployments_create

> models::Deployment function_deployments_create(endpoint_id, create_deployment_input)
Create immutable deployment and hot-reload runner

Uploads and registers an immutable code bundle in function.deployments, increments revision version, sets it as the active deployment pointer, and immediately hot-reloads runtime worker isolates. Emits function.deployment.created event. Requires function:endpoint.deploy scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**create_deployment_input** | [**CreateDeploymentInput**](CreateDeploymentInput.md) | Request body for function.CreateDeploymentInput | [required] |

### Return type

[**models::Deployment**](Deployment.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_deployments_list

> models::ListDeploymentsResponse function_deployments_list(endpoint_id)
List immutable deployments for an endpoint

Returns version history of immutable code deployments for an endpoint from function.deployments, detailing revision numbers, creation timestamps, and active flags. Requires function:endpoint.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |

### Return type

[**models::ListDeploymentsResponse**](ListDeploymentsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## function_deployments_rollback

> models::Deployment function_deployments_rollback(endpoint_id, rollback_deployment_input)
Rollback endpoint to a previous deployment revision

Atomically updates the active deployment pointer in function.endpoints to an earlier immutable deployment version, instantly hot-reloading runtime runner isolates. Emits function.deployment.rollback event. Requires function:endpoint.deploy scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**endpoint_id** | **String** |  | [required] |
**rollback_deployment_input** | [**RollbackDeploymentInput**](RollbackDeploymentInput.md) | Request body for function.RollbackDeploymentInput | [required] |

### Return type

[**models::Deployment**](Deployment.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

