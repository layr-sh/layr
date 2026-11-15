# \TablesPoliciesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_tables_policies_create**](TablesPoliciesApi.md#data_tables_policies_create) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/policies | Create a Row-Level Security policy on a table
[**data_tables_policies_delete**](TablesPoliciesApi.md#data_tables_policies_delete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name} | Drop an RLS policy from a table
[**data_tables_policies_list**](TablesPoliciesApi.md#data_tables_policies_list) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/policies | List Row-Level Security policies on a table



## data_tables_policies_create

> models::CreatePolicyResponse data_tables_policies_create(schema_name, table_name, create_policy_input)
Create a Row-Level Security policy on a table

Executes CREATE POLICY in PostgreSQL, attaching declarative row-level access control rules for specific database roles and SQL commands with USING and WITH CHECK expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**create_policy_input** | [**CreatePolicyInput**](CreatePolicyInput.md) | Request body for data.CreatePolicyInput | [required] |

### Return type

[**models::CreatePolicyResponse**](CreatePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_policies_delete

> models::DeletePolicyResponse data_tables_policies_delete(schema_name, table_name, policy_name)
Drop an RLS policy from a table

Executes DROP POLICY on the specified table, removing the security policy from PostgreSQL enforcement. Emits data.table.updated event. Requires data:schema.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |
**policy_name** | **String** |  | [required] |

### Return type

[**models::DeletePolicyResponse**](DeletePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## data_tables_policies_list

> models::ListPoliciesResponse data_tables_policies_list(schema_name, table_name)
List Row-Level Security policies on a table

Queries pg_policies to list active Row-Level Security (RLS) policies, detailing command targets (SELECT, INSERT, UPDATE, DELETE), roles, USING filters, and WITH CHECK expressions. Requires data:schema.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**schema_name** | **String** |  | [required] |
**table_name** | **String** |  | [required] |

### Return type

[**models::ListPoliciesResponse**](ListPoliciesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

