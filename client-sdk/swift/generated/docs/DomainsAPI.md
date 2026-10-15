# DomainsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**functionDomainsCreate**](DomainsAPI.md#functiondomainscreate) | **POST** /v1/_/function/domains | Register a standalone custom domain
[**functionDomainsDelete**](DomainsAPI.md#functiondomainsdelete) | **DELETE** /v1/_/function/domains/{domain_id} | Delete custom domain and cascade attached routes
[**functionDomainsGet**](DomainsAPI.md#functiondomainsget) | **GET** /v1/_/function/domains/{domain_id} | Get custom domain details and DNS instructions
[**functionDomainsList**](DomainsAPI.md#functiondomainslist) | **GET** /v1/_/function/domains | List registered custom domains
[**functionDomainsUpdate**](DomainsAPI.md#functiondomainsupdate) | **PATCH** /v1/_/function/domains/{domain_id} | Update custom domain configuration
[**functionDomainsVerify**](DomainsAPI.md#functiondomainsverify) | **POST** /v1/_/function/domains/{domain_id}/verify | Verify custom domain DNS configuration


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

let createCustomDomainInput = CreateCustomDomainInput(domain: "domain_example", tlsAcmeEmail: "tlsAcmeEmail_example", tlsEnabled: false) // CreateCustomDomainInput | Request body for function.CreateCustomDomainInput

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

# **functionDomainsGet**
```swift
    open class func functionDomainsGet(domainId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CustomDomain
```

Get custom domain details and DNS instructions

Returns a single custom domain definition by UUID from function.custom_domains including verification status, TLS configuration, and DNS setup instructions. Requires function:endpoint.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let domainId = "domainId_example" // String | 

// Get custom domain details and DNS instructions
do {
    let response = try await DomainsAPI.functionDomainsGet(domainId: domainId)
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

[**CustomDomain**](CustomDomain.md)

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

# **functionDomainsUpdate**
```swift
    open class func functionDomainsUpdate(domainId: String, updateCustomDomainInput: UpdateCustomDomainInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CustomDomain
```

Update custom domain configuration

Updates custom domain configuration such as toggling TLS on or off, or updating ACME contact email at runtime. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let domainId = "domainId_example" // String | 
let updateCustomDomainInput = UpdateCustomDomainInput(tlsAcmeEmail: "tlsAcmeEmail_example", tlsEnabled: false) // UpdateCustomDomainInput | Request body for function.UpdateCustomDomainInput

// Update custom domain configuration
do {
    let response = try await DomainsAPI.functionDomainsUpdate(domainId: domainId, updateCustomDomainInput: updateCustomDomainInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **domainId** | **String** |  | 
 **updateCustomDomainInput** | [**UpdateCustomDomainInput**](UpdateCustomDomainInput.md) | Request body for function.UpdateCustomDomainInput | 

### Return type

[**CustomDomain**](CustomDomain.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionDomainsVerify**
```swift
    open class func functionDomainsVerify(domainId: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CustomDomain
```

Verify custom domain DNS configuration

Performs a live DNS lookup (CNAME/IP) to verify domain DNS points to Layr ingress. If matched, activates domain status and triggers TLS provisioning when enabled. If not matched, returns misconfigured status with diagnostic error details. Requires function:endpoint.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let domainId = "domainId_example" // String | 
let body =  // JSONValue | Request body for core.Empty

// Verify custom domain DNS configuration
do {
    let response = try await DomainsAPI.functionDomainsVerify(domainId: domainId, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **domainId** | **String** |  | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**CustomDomain**](CustomDomain.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

