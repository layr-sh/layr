# TablesPoliciesApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataTablesPoliciesCreate**](TablesPoliciesApi.md#datatablespoliciescreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/policies | Create a Row-Level Security policy on a table |
| [**dataTablesPoliciesDelete**](TablesPoliciesApi.md#datatablespoliciesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name} | Drop an RLS policy from a table |
| [**dataTablesPoliciesList**](TablesPoliciesApi.md#datatablespolicieslist) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/policies | List Row-Level Security policies on a table |



## dataTablesPoliciesCreate

> CreatePolicyResponse dataTablesPoliciesCreate(schemaName, tableName, createPolicyInput)

Create a Row-Level Security policy on a table

Executes CREATE POLICY in PostgreSQL, attaching declarative row-level access control rules for specific database roles and SQL commands with USING and WITH CHECK expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesPoliciesApi,
} from '';
import type { DataTablesPoliciesCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesPoliciesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // CreatePolicyInput | Request body for data.CreatePolicyInput
    createPolicyInput: ...,
  } satisfies DataTablesPoliciesCreateRequest;

  try {
    const data = await api.dataTablesPoliciesCreate(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **schemaName** | `string` |  | [Defaults to `undefined`] |
| **tableName** | `string` |  | [Defaults to `undefined`] |
| **createPolicyInput** | [CreatePolicyInput](CreatePolicyInput.md) | Request body for data.CreatePolicyInput | |

### Return type

[**CreatePolicyResponse**](CreatePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataTablesPoliciesDelete

> DeletePolicyResponse dataTablesPoliciesDelete(schemaName, tableName, policyName)

Drop an RLS policy from a table

Executes DROP POLICY on the specified table, removing the security policy from PostgreSQL enforcement. Emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesPoliciesApi,
} from '';
import type { DataTablesPoliciesDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesPoliciesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    policyName: policyName_example,
  } satisfies DataTablesPoliciesDeleteRequest;

  try {
    const data = await api.dataTablesPoliciesDelete(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **schemaName** | `string` |  | [Defaults to `undefined`] |
| **tableName** | `string` |  | [Defaults to `undefined`] |
| **policyName** | `string` |  | [Defaults to `undefined`] |

### Return type

[**DeletePolicyResponse**](DeletePolicyResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataTablesPoliciesList

> ListPoliciesResponse dataTablesPoliciesList(schemaName, tableName)

List Row-Level Security policies on a table

Queries pg_policies to list active Row-Level Security (RLS) policies, detailing command targets (SELECT, INSERT, UPDATE, DELETE), roles, USING filters, and WITH CHECK expressions. Requires data:schema.read scope.

### Example

```ts
import {
  Configuration,
  TablesPoliciesApi,
} from '';
import type { DataTablesPoliciesListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesPoliciesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
  } satisfies DataTablesPoliciesListRequest;

  try {
    const data = await api.dataTablesPoliciesList(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **schemaName** | `string` |  | [Defaults to `undefined`] |
| **tableName** | `string` |  | [Defaults to `undefined`] |

### Return type

[**ListPoliciesResponse**](ListPoliciesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

