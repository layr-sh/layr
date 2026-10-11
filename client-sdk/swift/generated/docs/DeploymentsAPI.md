# DeploymentsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionDeploymentsCreate**](DeploymentsAPI.md#functiondeploymentscreate) | **POST** /v1/_/function/endpoints/{endpoint_id}/deploy | Create immutable deployment and hot-reload runner
[**functionDeploymentsList**](DeploymentsAPI.md#functiondeploymentslist) | **GET** /v1/_/function/endpoints/{endpoint_id}/deployments | List immutable deployments for an endpoint
[**functionDeploymentsRollback**](DeploymentsAPI.md#functiondeploymentsrollback) | **POST** /v1/_/function/endpoints/{endpoint_id}/rollback | Rollback endpoint to a previous deployment revision


# **functionDeploymentsCreate**
```swift
    open class func functionDeploymentsCreate(endpointId: String, createDeploymentInput: CreateDeploymentInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Deployment
```

Create immutable deployment and hot-reload runner

Uploads and registers an immutable code bundle in function.deployments, increments revision version, sets it as the active deployment pointer, and immediately hot-reloads runtime worker isolates. Emits function.deployment.created event. Requires function:endpoint.deploy scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let createDeploymentInput = CreateDeploymentInput(bundleContent: "bundleContent_example", bundleFiles: "TODO", bundleFormat: "bundleFormat_example", environmentVariables: "TODO", workerdRuntimeConfig: WorkerdRuntimeConfig(compatibilityDate: "compatibilityDate_example", compatibilityFlags: ["compatibilityFlags_example"])) // CreateDeploymentInput | Request body for function.CreateDeploymentInput

// Create immutable deployment and hot-reload runner
do {
    let response = try await DeploymentsAPI.functionDeploymentsCreate(endpointId: endpointId, createDeploymentInput: createDeploymentInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **createDeploymentInput** | [**CreateDeploymentInput**](CreateDeploymentInput.md) | Request body for function.CreateDeploymentInput | 

### Return type

[**Deployment**](Deployment.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionDeploymentsList**
```swift
    open class func functionDeploymentsList(endpointId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListDeploymentsResponse
```

List immutable deployments for an endpoint

Returns version history of immutable code deployments for an endpoint from function.deployments, detailing revision numbers, creation timestamps, and active flags. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 

// List immutable deployments for an endpoint
do {
    let response = try await DeploymentsAPI.functionDeploymentsList(endpointId: endpointId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 

### Return type

[**ListDeploymentsResponse**](ListDeploymentsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionDeploymentsRollback**
```swift
    open class func functionDeploymentsRollback(endpointId: String, rollbackDeploymentInput: RollbackDeploymentInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Deployment
```

Rollback endpoint to a previous deployment revision

Atomically updates the active deployment pointer in function.endpoints to an earlier immutable deployment version, instantly hot-reloading runtime runner isolates. Emits function.deployment.rollback event. Requires function:endpoint.deploy scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let rollbackDeploymentInput = RollbackDeploymentInput(targetVersion: 123) // RollbackDeploymentInput | Request body for function.RollbackDeploymentInput

// Rollback endpoint to a previous deployment revision
do {
    let response = try await DeploymentsAPI.functionDeploymentsRollback(endpointId: endpointId, rollbackDeploymentInput: rollbackDeploymentInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **rollbackDeploymentInput** | [**RollbackDeploymentInput**](RollbackDeploymentInput.md) | Request body for function.RollbackDeploymentInput | 

### Return type

[**Deployment**](Deployment.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

