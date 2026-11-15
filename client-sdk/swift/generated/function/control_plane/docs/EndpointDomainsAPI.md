# EndpointDomainsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionEndpointDomainsCreate**](EndpointDomainsAPI.md#functionendpointdomainscreate) | **POST** /v1/_/function/endpoints/{endpoint_id}/domains | Attach endpoint route to custom domain with path prefix
[**functionEndpointDomainsDelete**](EndpointDomainsAPI.md#functionendpointdomainsdelete) | **DELETE** /v1/_/function/endpoints/{endpoint_id}/domains/{route_id} | Remove endpoint route mapping from custom domain
[**functionEndpointDomainsList**](EndpointDomainsAPI.md#functionendpointdomainslist) | **GET** /v1/_/function/endpoints/{endpoint_id}/domains | List custom domain routes attached to an endpoint


# **functionEndpointDomainsCreate**
```swift
    open class func functionEndpointDomainsCreate(endpointId: String, createCustomDomainRouteInput: CreateCustomDomainRouteInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CustomDomainRoute
```

Attach endpoint route to custom domain with path prefix

Maps a custom domain hostname and URL path prefix to a specific serverless endpoint in function.custom_domain_routes, enabling custom domain routing. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let createCustomDomainRouteInput = CreateCustomDomainRouteInput(customDomainId: 123, pathPrefix: "pathPrefix_example") // CreateCustomDomainRouteInput | Request body for function.CreateCustomDomainRouteInput

// Attach endpoint route to custom domain with path prefix
do {
    let response = try await EndpointDomainsAPI.functionEndpointDomainsCreate(endpointId: endpointId, createCustomDomainRouteInput: createCustomDomainRouteInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **createCustomDomainRouteInput** | [**CreateCustomDomainRouteInput**](CreateCustomDomainRouteInput.md) | Request body for function.CreateCustomDomainRouteInput | 

### Return type

[**CustomDomainRoute**](CustomDomainRoute.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointDomainsDelete**
```swift
    open class func functionEndpointDomainsDelete(endpointId: String, routeId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Remove endpoint route mapping from custom domain

Removes a specific domain route mapping from function.custom_domain_routes, unbinding the custom path prefix from the endpoint. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 
let routeId = "routeId_example" // String | 

// Remove endpoint route mapping from custom domain
do {
    let response = try await EndpointDomainsAPI.functionEndpointDomainsDelete(endpointId: endpointId, routeId: routeId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **endpointId** | **String** |  | 
 **routeId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionEndpointDomainsList**
```swift
    open class func functionEndpointDomainsList(endpointId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListCustomDomainRoutesResponse
```

List custom domain routes attached to an endpoint

Returns all custom domain route mappings from function.custom_domain_routes configured for a specific serverless endpoint. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let endpointId = "endpointId_example" // String | 

// List custom domain routes attached to an endpoint
do {
    let response = try await EndpointDomainsAPI.functionEndpointDomainsList(endpointId: endpointId)
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

[**ListCustomDomainRoutesResponse**](ListCustomDomainRoutesResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

