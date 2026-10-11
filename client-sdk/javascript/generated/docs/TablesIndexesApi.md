# TablesIndexesApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataTablesIndexesCreate**](TablesIndexesApi.md#datatablesindexescreate) | **POST** /v1/_/data/tables/{schema_name}/{table_name}/indexes | Create an index on table columns |
| [**dataTablesIndexesDelete**](TablesIndexesApi.md#datatablesindexesdelete) | **DELETE** /v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name} | Drop an index from a table |
| [**dataTablesIndexesList**](TablesIndexesApi.md#datatablesindexeslist) | **GET** /v1/_/data/tables/{schema_name}/{table_name}/indexes | List indexes defined on a table |



## dataTablesIndexesCreate

> CreateIndexResponse dataTablesIndexesCreate(schemaName, tableName, createIndexInput)

Create an index on table columns

Executes CREATE INDEX supporting B-Tree, GIN, GiST, or BRIN index methods, unique constraints, and multi-column expressions. Emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesIndexesApi,
} from '';
import type { DataTablesIndexesCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesIndexesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // CreateIndexInput | Request body for data.CreateIndexInput
    createIndexInput: ...,
  } satisfies DataTablesIndexesCreateRequest;

  try {
    const data = await api.dataTablesIndexesCreate(body);
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
| **createIndexInput** | [CreateIndexInput](CreateIndexInput.md) | Request body for data.CreateIndexInput | |

### Return type

[**CreateIndexResponse**](CreateIndexResponse.md)

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


## dataTablesIndexesDelete

> DeleteIndexResponse dataTablesIndexesDelete(schemaName, tableName, indexName)

Drop an index from a table

Executes DROP INDEX in PostgreSQL, removing the index and invalidating query planner metadata. Emits data.table.updated event. Requires data:schema.write scope.

### Example

```ts
import {
  Configuration,
  TablesIndexesApi,
} from '';
import type { DataTablesIndexesDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesIndexesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    indexName: indexName_example,
  } satisfies DataTablesIndexesDeleteRequest;

  try {
    const data = await api.dataTablesIndexesDelete(body);
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
| **indexName** | `string` |  | [Defaults to `undefined`] |

### Return type

[**DeleteIndexResponse**](DeleteIndexResponse.md)

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


## dataTablesIndexesList

> ListIndexesResponse dataTablesIndexesList(schemaName, tableName)

List indexes defined on a table

Queries pg_indexes to return all index definitions, methods, uniqueness constraints, and column lists for the specified table. Requires data:schema.read scope.

### Example

```ts
import {
  Configuration,
  TablesIndexesApi,
} from '';
import type { DataTablesIndexesListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new TablesIndexesApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
  } satisfies DataTablesIndexesListRequest;

  try {
    const data = await api.dataTablesIndexesList(body);
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

[**ListIndexesResponse**](ListIndexesResponse.md)

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

