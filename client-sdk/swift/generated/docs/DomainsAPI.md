# DomainsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionDomainsCreate**](DomainsAPI.md#functiondomainscreate) | **POST** /v1/_/function/domains | Register a standalone custom domain
[**functionDomainsDelete**](DomainsAPI.md#functiondomainsdelete) | **DELETE** /v1/_/function/domains/{domain_id} | Delete custom domain and cascade attached routes
[**functionDomainsList**](DomainsAPI.md#functiondomainslist) | **GET** /v1/_/function/domains | List registered custom domains


# **functionDomainsCreate**
```swift
    open class func functionDomainsCreate(createCustomDomainInput: CreateCustomDomainInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CustomDomain
```

Register a standalone custom domain

Registers a standalone custom domain hostname in function.custom_domains with verification status and TLS configuration. Emits function.domain.created event. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createCustomDomainInput = CreateCustomDomainInput(domain: "domain_example") // CreateCustomDomainInput | Request body for function.CreateCustomDomainInput

// Register a standalone custom domain
do {
    let response = try await DomainsAPI.functionDomainsCreate(createCustomDomainInput: createCustomDomainInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createCustomDomainInput** | [**CreateCustomDomainInput**](CreateCustomDomainInput.md) | Request body for function.CreateCustomDomainInput | 

### Return type

[**CustomDomain**](CustomDomain.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionDomainsDelete**
```swift
    open class func functionDomainsDelete(domainId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete custom domain and cascade attached routes

Deletes a custom domain from function.custom_domains, cascading removal of all attached endpoint route mappings. Emits function.domain.deleted event. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let domainId = "domainId_example" // String | 

// Delete custom domain and cascade attached routes
do {
    let response = try await DomainsAPI.functionDomainsDelete(domainId: domainId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **domainId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionDomainsList**
```swift
    open class func functionDomainsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListCustomDomainsResponse
```

List registered custom domains

Returns all registered custom domain definitions from function.custom_domains with hostnames and verification states. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List registered custom domains
do {
    let response = try await DomainsAPI.functionDomainsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListCustomDomainsResponse**](ListCustomDomainsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

