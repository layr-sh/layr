# RecordsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataRecordsCreate**](RecordsApi.md#datarecordscreate) | **POST** /v1/data/{schema_name}/{table_name} | Insert single or bulk rows with RETURNING * |
| [**dataRecordsDelete**](RecordsApi.md#datarecordsdelete) | **DELETE** /v1/data/{schema_name}/{table_name}/{record_id} | Delete a row by primary key |
| [**dataRecordsGet**](RecordsApi.md#datarecordsget) | **GET** /v1/data/{schema_name}/{table_name}/{record_id} | Get a single row by primary key |
| [**dataRecordsList**](RecordsApi.md#datarecordslist) | **GET** /v1/data/{schema_name}/{table_name} | Query table rows with dynamic filtering, sorting, and pagination |
| [**dataRecordsPut**](RecordsApi.md#datarecordsput) | **PUT** /v1/data/{schema_name}/{table_name}/{record_id} | Update entire row by primary key |
| [**dataRecordsUpdate**](RecordsApi.md#datarecordsupdate) | **PATCH** /v1/data/{schema_name}/{table_name}/{record_id} | Partially update a row by primary key |



## dataRecordsCreate

> CreateRecordResponse dataRecordsCreate(schemaName, tableName, createRecordInput)

Insert single or bulk rows with RETURNING *

Inserts a single row or batch of JSON objects into PostgreSQL, automatically applying defaults and returning inserted tuples. Validates against RLS INSERT ... WITH CHECK policies, invalidates table query caches, and emits real-time CDC updates.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // CreateRecordInput | Request body for data.CreateRecordInput
    createRecordInput: ...,
  } satisfies DataRecordsCreateRequest;

  try {
    const data = await api.dataRecordsCreate(body);
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
| **createRecordInput** | [CreateRecordInput](CreateRecordInput.md) | Request body for data.CreateRecordInput | |

### Return type

[**CreateRecordResponse**](CreateRecordResponse.md)

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


## dataRecordsDelete

> any dataRecordsDelete(schemaName, tableName, recordId)

Delete a row by primary key

Deletes a row by primary key from PostgreSQL subject to caller RLS DELETE ... USING policies, invalidating table query caches and broadcasting CDC events.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    recordId: recordId_example,
  } satisfies DataRecordsDeleteRequest;

  try {
    const data = await api.dataRecordsDelete(body);
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
| **recordId** | `string` |  | [Defaults to `undefined`] |

### Return type

**any**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **204** | Row deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataRecordsGet

> GetRecordResponse dataRecordsGet(schemaName, tableName, recordId)

Get a single row by primary key

Retrieves a single row from PostgreSQL by UUID or primary key, enforcing caller Row-Level Security (RLS) SELECT policies and executing within caller session claims.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    recordId: recordId_example,
  } satisfies DataRecordsGetRequest;

  try {
    const data = await api.dataRecordsGet(body);
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
| **recordId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**GetRecordResponse**](GetRecordResponse.md)

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


## dataRecordsList

> ListRecordsResponse dataRecordsList(schemaName, tableName)

Query table rows with dynamic filtering, sorting, and pagination

Queries PostgreSQL table rows with dynamic filter predicates (eq, gte, in, like), JSON path filtering, relational foreign-key embedding, cursor/offset pagination, and multi-column sorting. Injects caller JWT session claims into transaction-local variables for native Row-Level Security (RLS) enforcement. Supports optional KV result caching via ?cache_ttl.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
  } satisfies DataRecordsListRequest;

  try {
    const data = await api.dataRecordsList(body);
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

[**ListRecordsResponse**](ListRecordsResponse.md)

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


## dataRecordsPut

> UpdateRecordResponse dataRecordsPut(schemaName, tableName, recordId, updateRecordInput)

Update entire row by primary key

Replaces column values for a row identified by primary key, validating against RLS UPDATE policies, invalidating table query caches, and broadcasting CDC changes.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsPutRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    recordId: recordId_example,
    // UpdateRecordInput | Request body for data.UpdateRecordInput
    updateRecordInput: ...,
  } satisfies DataRecordsPutRequest;

  try {
    const data = await api.dataRecordsPut(body);
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
| **recordId** | `string` |  | [Defaults to `undefined`] |
| **updateRecordInput** | [UpdateRecordInput](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | |

### Return type

[**UpdateRecordResponse**](UpdateRecordResponse.md)

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


## dataRecordsUpdate

> UpdateRecordResponse dataRecordsUpdate(schemaName, tableName, recordId, updateRecordInput)

Partially update a row by primary key

Applies partial column updates to a row identified by primary key, validating against RLS UPDATE ... USING ... WITH CHECK policies, invalidating cached queries, and broadcasting CDC changes.

### Example

```ts
import {
  Configuration,
  RecordsApi,
} from '';
import type { DataRecordsUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RecordsApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    tableName: tableName_example,
    // string
    recordId: recordId_example,
    // UpdateRecordInput | Request body for data.UpdateRecordInput
    updateRecordInput: ...,
  } satisfies DataRecordsUpdateRequest;

  try {
    const data = await api.dataRecordsUpdate(body);
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
| **recordId** | `string` |  | [Defaults to `undefined`] |
| **updateRecordInput** | [UpdateRecordInput](UpdateRecordInput.md) | Request body for data.UpdateRecordInput | |

### Return type

[**UpdateRecordResponse**](UpdateRecordResponse.md)

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

