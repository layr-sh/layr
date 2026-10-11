# TablesApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataTablesCreate**](TablesApi.md#datatablescreate) | **POST** /v1/_/data/tables | Create a new PostgreSQL table with UUIDv7 primary key |
| [**dataTablesDelete**](TablesApi.md#datatablesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name} | Drop a table and cascade constraints |
| [**dataTablesGet**](TablesApi.md#datatablesget) | **GET** /v1/_/data/tables/{schema_name}/{table_name} | Inspect table schema, columns, indexes, and constraints |
| [**dataTablesList**](TablesApi.md#datatableslist) | **GET** /v1/_/data/tables | List all database tables across configured schemas |



## dataTablesCreate

> CreateTableResponse dataTablesCreate(createTableInput)

Create a new PostgreSQL table with UUIDv7 primary key

Executes DDL to provision a table adhering to Layr conventions (UUIDv7 primary keys, created_at, and last_updated_at timestamps), invalidating catalog caches and emitting data.table.created event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesApi,
} from '';
import type { DataTablesCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesApi();

  const body = {
    // CreateTableInput | Request body for data.CreateTableInput
    createTableInput: ...,
  } satisfies DataTablesCreateRequest;

  try {
    const data = await api.dataTablesCreate(body);
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
| **createTableInput** | [CreateTableInput](CreateTableInput.md) | Request body for data.CreateTableInput | |

### Return type

[**CreateTableResponse**](CreateTableResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataTablesDelete

> DeleteTableResponse dataTablesDelete(schemaName, tableName, cascade)

Drop a table and cascade constraints

Executes DROP TABLE CASCADE in PostgreSQL, removing the table, associated indexes, triggers, and foreign key constraints. Invalidates catalog caches and emits data.table.deleted event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesApi,
} from '';
import type { DataTablesDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // boolean | Cascade table drop to dependent constraints and views (optional)
    cascade: true,
  } satisfies DataTablesDeleteRequest;

  try {
    const data = await api.dataTablesDelete(body);
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
| **cascade** | `boolean` | Cascade table drop to dependent constraints and views | [Optional] [Defaults to `undefined`] |

### Return type

[**DeleteTableResponse**](DeleteTableResponse.md)

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


## dataTablesGet

> Table dataTablesGet(schemaName, tableName)

Inspect table schema, columns, indexes, and constraints

Retrieves complete architectural schema for a table, detailing columns (types, nullability, defaults), primary/foreign key constraints, indexes, and active RLS policies. Requires data:schema.read scope.

### Example

```ts
import {
  Configuration,
  TablesApi,
} from '';
import type { DataTablesGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
  } satisfies DataTablesGetRequest;

  try {
    const data = await api.dataTablesGet(body);
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

[**Table**](Table.md)

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


## dataTablesList

> ListTablesResponse dataTablesList()

List all database tables across configured schemas

Queries PostgreSQL information_schema and pg_catalog to return tables, primary key column names, approximate row counts, and RLS enabled statuses across exposed schemas. Requires data:schema.read scope.

### Example

```ts
import {
  Configuration,
  TablesApi,
} from '';
import type { DataTablesListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesApi();

  try {
    const data = await api.dataTablesList();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**ListTablesResponse**](ListTablesResponse.md)

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

