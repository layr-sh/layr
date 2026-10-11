# TablesRLSApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataTablesRlsUpdate**](TablesRLSApi.md#datatablesrlsupdate) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/rls | Toggle Row-Level Security enforcement mode on a table |



## dataTablesRlsUpdate

> ToggleRLSResponse dataTablesRlsUpdate(schemaName, tableName, toggleRLSInput, action, force)

Toggle Row-Level Security enforcement mode on a table

Executes ALTER TABLE to ENABLE, DISABLE, or FORCE Row-Level Security (RLS) enforcement on the table. Invalidates table metadata and emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesRLSApi,
} from '';
import type { DataTablesRlsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesRLSApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // ToggleRLSInput | Request body for data.ToggleRLSInput
    toggleRLSInput: ...,
    // string | RLS enforcement mode (\'ENABLE\', \'DISABLE\', or \'FORCE\') (optional)
    action: action_example,
    // boolean | Whether to enforce RLS for table owners when action is FORCE (optional)
    force: true,
  } satisfies DataTablesRlsUpdateRequest;

  try {
    const data = await api.dataTablesRlsUpdate(body);
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
| **toggleRLSInput** | [ToggleRLSInput](ToggleRLSInput.md) | Request body for data.ToggleRLSInput | |
| **action** | `string` | RLS enforcement mode (\&#39;ENABLE\&#39;, \&#39;DISABLE\&#39;, or \&#39;FORCE\&#39;) | [Optional] [Defaults to `undefined`] |
| **force** | `boolean` | Whether to enforce RLS for table owners when action is FORCE | [Optional] [Defaults to `undefined`] |

### Return type

[**ToggleRLSResponse**](ToggleRLSResponse.md)

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

