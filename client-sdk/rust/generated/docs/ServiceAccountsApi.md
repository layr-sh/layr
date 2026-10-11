# \ServiceAccountsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_service_accounts_create**](ServiceAccountsApi.md#core_service_accounts_create) | **POST** /v1/_/core/service-accounts | Create a machine service account
[**core_service_accounts_delete**](ServiceAccountsApi.md#core_service_accounts_delete) | **DELETE** /v1/_/core/service-accounts/{service_account_id} | Delete machine service account
[**core_service_accounts_get**](ServiceAccountsApi.md#core_service_accounts_get) | **GET** /v1/_/core/service-accounts/{service_account_id} | Get machine service account details by UUID
[**core_service_accounts_list**](ServiceAccountsApi.md#core_service_accounts_list) | **GET** /v1/_/core/service-accounts | List machine service accounts
[**core_service_accounts_update**](ServiceAccountsApi.md#core_service_accounts_update) | **PUT** /v1/_/core/service-accounts/{service_account_id} | Update machine service account properties and scopes



## core_service_accounts_create

> models::CreateServiceAccountResponse core_service_accounts_create(create_service_account_input)
Create a machine service account

Creates a machine service account, generates a high-entropy 32-byte hex secret key, hashes it with SHA-256 for storage in core.service_accounts, and returns the plaintext secret key once. Supports scope constraints and CIDR IP restrictions. Requires core:service-account.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_service_account_input** | [**CreateServiceAccountInput**](CreateServiceAccountInput.md) | Request body for core.CreateServiceAccountInput | [required] |

### Return type

[**models::CreateServiceAccountResponse**](CreateServiceAccountResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_service_accounts_delete

> serde_json::Value core_service_accounts_delete(service_account_id)
Delete machine service account

Deletes a machine service account from core.service_accounts. Enforces strict safety invariants: prevents deleting accounts owned by console users and ensures at least one active root account remains. Requires core:service-account.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**service_account_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_service_accounts_get

> models::ServiceAccount core_service_accounts_get(service_account_id)
Get machine service account details by UUID

Retrieves a machine service account by UUID from core.service_accounts, returning metadata, permission scopes, key prefix, and allowed IP patterns. Plaintext key is not recoverable. Requires core:service-account.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**service_account_id** | **String** |  | [required] |

### Return type

[**models::ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_service_accounts_list

> Vec<models::ServiceAccount> core_service_accounts_list()
List machine service accounts

Lists all machine service accounts from core.service_accounts with name, key prefix, granted permission scopes, enabled status, IP allowlists, and expiration timestamps. Requires core:service-account.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**Vec<models::ServiceAccount>**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_service_accounts_update

> models::ServiceAccount core_service_accounts_update(service_account_id, update_service_account_input)
Update machine service account properties and scopes

Modifies name, description, permission scopes, IP allowlist, or enabled status in core.service_accounts. Enforces safety invariant that at least one enabled root account with wildcard * scope remains. Requires core:service-account.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**service_account_id** | **String** |  | [required] |
**update_service_account_input** | [**UpdateServiceAccountInput**](UpdateServiceAccountInput.md) | Request body for core.UpdateServiceAccountInput | [required] |

### Return type

[**models::ServiceAccount**](ServiceAccount.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

