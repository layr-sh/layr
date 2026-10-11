# TablesColumnsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataTablesColumnsCreate**](TablesColumnsApi.md#datatablescolumnscreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/columns | Add a new column to a table schema |
| [**dataTablesColumnsDelete**](TablesColumnsApi.md#datatablescolumnsdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Drop a column from a table schema |
| [**dataTablesColumnsUpdate**](TablesColumnsApi.md#datatablescolumnsupdate) | **PATCH** /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name} | Alter column type, default, nullability, or rename |



## dataTablesColumnsCreate

> CreateColumnResponse dataTablesColumnsCreate(schemaName, tableName, column)

Add a new column to a table schema

Executes ALTER TABLE ADD COLUMN with validated PostgreSQL data type, nullability constraint, and optional default expression. Invalidates schema reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesColumnsApi,
} from '';
import type { DataTablesColumnsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesColumnsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // Column | Request body for data.Column
    column: ...,
  } satisfies DataTablesColumnsCreateRequest;

  try {
    const data = await api.dataTablesColumnsCreate(body);
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
| **column** | [Column](Column.md) | Request body for data.Column | |

### Return type

[**CreateColumnResponse**](CreateColumnResponse.md)

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


## dataTablesColumnsDelete

> DeleteColumnResponse dataTablesColumnsDelete(schemaName, tableName, columnName)

Drop a column from a table schema

Executes ALTER TABLE DROP COLUMN CASCADE, dropping the column and dependent views or indexes. Invalidates reflection caches and emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesColumnsApi,
} from '';
import type { DataTablesColumnsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesColumnsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    columnName: columnName_example,
  } satisfies DataTablesColumnsDeleteRequest;

  try {
    const data = await api.dataTablesColumnsDelete(body);
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
| **columnName** | `string` |  | [Defaults to `undefined`] |

### Return type

[**DeleteColumnResponse**](DeleteColumnResponse.md)

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


## dataTablesColumnsUpdate

> UpdateColumnResponse dataTablesColumnsUpdate(schemaName, tableName, columnName, updateColumnInput)

Alter column type, default, nullability, or rename

Alters column attributes (data type conversion, set/drop default, set/drop NOT NULL) or renames the column. Invalidates table metadata caches and emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesColumnsApi,
} from '';
import type { DataTablesColumnsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesColumnsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    columnName: columnName_example,
    // UpdateColumnInput | Request body for data.UpdateColumnInput
    updateColumnInput: ...,
  } satisfies DataTablesColumnsUpdateRequest;

  try {
    const data = await api.dataTablesColumnsUpdate(body);
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
| **columnName** | `string` |  | [Defaults to `undefined`] |
| **updateColumnInput** | [UpdateColumnInput](UpdateColumnInput.md) | Request body for data.UpdateColumnInput | |

### Return type

[**UpdateColumnResponse**](UpdateColumnResponse.md)

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

