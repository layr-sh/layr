# RPCApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataRpcMutation**](RPCApi.md#datarpcmutation) | **POST** /v1/data/{schema_name}/rpc/{function_name} | Execute database stored function or procedure with JSON payload |
| [**dataRpcQuery**](RPCApi.md#datarpcquery) | **GET** /v1/data/{schema_name}/rpc/{function_name} | Execute read-only database stored function or procedure |



## dataRpcMutation

> any dataRpcMutation(schemaName, functionName, body, invalidateTables)

Execute database stored function or procedure with JSON payload

Invokes a PostgreSQL stored procedure or function with JSON request body arguments, setting caller RLS session claims and returning execution results.

### Example

```ts
import {
  Configuration,
  RPCApi,
} from '';
import type { DataRpcMutationRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RPCApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    functionName: functionName_example,
    // object | Request body for data.ExecuteFunctionInput
    body: Object,
    // string | Comma-separated table names whose cache should be invalidated (optional)
    invalidateTables: invalidateTables_example,
  } satisfies DataRpcMutationRequest;

  try {
    const data = await api.dataRpcMutation(body);
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
| **functionName** | `string` |  | [Defaults to `undefined`] |
| **body** | `object` | Request body for data.ExecuteFunctionInput | |
| **invalidateTables** | `string` | Comma-separated table names whose cache should be invalidated | [Optional] [Defaults to `undefined`] |

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

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## dataRpcQuery

> any dataRpcQuery(schemaName, functionName, invalidateTables)

Execute read-only database stored function or procedure

Invokes a PostgreSQL stored function or procedure with arguments passed via query parameters, setting transaction-local caller RLS session claims.

### Example

```ts
import {
  Configuration,
  RPCApi,
} from '';
import type { DataRpcQueryRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RPCApi();

  const body = {
    // string
    schemaName: schemaName_example,
    // string
    functionName: functionName_example,
    // string | Comma-separated table names whose cache should be invalidated (optional)
    invalidateTables: invalidateTables_example,
  } satisfies DataRpcQueryRequest;

  try {
    const data = await api.dataRpcQuery(body);
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
| **functionName** | `string` |  | [Defaults to `undefined`] |
| **invalidateTables** | `string` | Comma-separated table names whose cache should be invalidated | [Optional] [Defaults to `undefined`] |

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

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

