# ServiceAccountsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreServiceAccountsCreate**](ServiceAccountsAPI.md#coreserviceaccountscreate) | **POST** /v1/_/core/service-accounts | Create a machine service account
[**coreServiceAccountsDelete**](ServiceAccountsAPI.md#coreserviceaccountsdelete) | **DELETE** /v1/_/core/service-accounts/{service_account_id} | Delete machine service account
[**coreServiceAccountsGet**](ServiceAccountsAPI.md#coreserviceaccountsget) | **GET** /v1/_/core/service-accounts/{service_account_id} | Get machine service account details by UUID
[**coreServiceAccountsList**](ServiceAccountsAPI.md#coreserviceaccountslist) | **GET** /v1/_/core/service-accounts | List machine service accounts
[**coreServiceAccountsUpdate**](ServiceAccountsAPI.md#coreserviceaccountsupdate) | **PUT** /v1/_/core/service-accounts/{service_account_id} | Update machine service account properties and scopes


# **coreServiceAccountsCreate**
```swift
    open class func coreServiceAccountsCreate(createServiceAccountInput: CreateServiceAccountInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> CreateServiceAccountResponse
```

Create a machine service account

Creates a machine service account, generates a high-entropy 32-byte hex secret key, hashes it with SHA-256 for storage in core.service_accounts, and returns the plaintext secret key once. Supports scope constraints and CIDR IP restrictions. Requires core:service-account.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createServiceAccountInput = CreateServiceAccountInput(allowedIps: ["allowedIps_example"], consoleUserId: "consoleUserId_example", description: "description_example", expiresAt: Date(), name: "name_example", scopes: ["scopes_example"]) // CreateServiceAccountInput | Request body for core.CreateServiceAccountInput

// Create a machine service account
do {
    let response = try await ServiceAccountsAPI.coreServiceAccountsCreate(createServiceAccountInput: createServiceAccountInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createServiceAccountInput** | [**CreateServiceAccountInput**](CreateServiceAccountInput.md) | Request body for core.CreateServiceAccountInput | 

### Return type

[**CreateServiceAccountResponse**](CreateServiceAccountResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreServiceAccountsDelete**
```swift
    open class func coreServiceAccountsDelete(serviceAccountId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete machine service account

Deletes a machine service account from core.service_accounts. Enforces strict safety invariants: prevents deleting accounts owned by console users and ensures at least one active root account remains. Requires core:service-account.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let serviceAccountId = "serviceAccountId_example" // String | 

// Delete machine service account
do {
    let response = try await ServiceAccountsAPI.coreServiceAccountsDelete(serviceAccountId: serviceAccountId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **serviceAccountId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreServiceAccountsGet**
```swift
    open class func coreServiceAccountsGet(serviceAccountId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ServiceAccount
```

Get machine service account details by UUID

Retrieves a machine service account by UUID from core.service_accounts, returning metadata, permission scopes, key prefix, and allowed IP patterns. Plaintext key is not recoverable. Requires core:service-account.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let serviceAccountId = "serviceAccountId_example" // String | 

// Get machine service account details by UUID
do {
    let response = try await ServiceAccountsAPI.coreServiceAccountsGet(serviceAccountId: serviceAccountId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **serviceAccountId** | **String** |  | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreServiceAccountsList**
```swift
    open class func coreServiceAccountsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [ServiceAccount]
```

List machine service accounts

Lists all machine service accounts from core.service_accounts with name, key prefix, granted permission scopes, enabled status, IP allowlists, and expiration timestamps. Requires core:service-account.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List machine service accounts
do {
    let response = try await ServiceAccountsAPI.coreServiceAccountsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**[ServiceAccount]**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreServiceAccountsUpdate**
```swift
    open class func coreServiceAccountsUpdate(serviceAccountId: String, updateServiceAccountInput: UpdateServiceAccountInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ServiceAccount
```

Update machine service account properties and scopes

Modifies name, description, permission scopes, IP allowlist, or enabled status in core.service_accounts. Enforces safety invariant that at least one enabled root account with wildcard * scope remains. Requires core:service-account.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let serviceAccountId = "serviceAccountId_example" // String | 
let updateServiceAccountInput = UpdateServiceAccountInput(allowedIps: ["allowedIps_example"], description: "description_example", expiresAt: Date(), isEnabled: false, name: "name_example", scopes: ["scopes_example"]) // UpdateServiceAccountInput | Request body for core.UpdateServiceAccountInput

// Update machine service account properties and scopes
do {
    let response = try await ServiceAccountsAPI.coreServiceAccountsUpdate(serviceAccountId: serviceAccountId, updateServiceAccountInput: updateServiceAccountInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **serviceAccountId** | **String** |  | 
 **updateServiceAccountInput** | [**UpdateServiceAccountInput**](UpdateServiceAccountInput.md) | Request body for core.UpdateServiceAccountInput | 

### Return type

[**ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

