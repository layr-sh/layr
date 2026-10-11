# GraphQLApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**dataGraphql**](GraphQLApi.md#datagraphql) | **POST** /v1/graphql | Execute GraphQL query or mutation |



## dataGraphql

> ExecuteGraphQLResponse dataGraphql(executeGraphQLInput)

Execute GraphQL query or mutation

Dynamic introspection and execution engine over PostgreSQL tables, supporting complex nested selections, arguments, variables, query depth limits (max depth guard), complexity budgets, and native RLS session context injection.

### Example

```ts
import {
  Configuration,
  GraphQLApi,
} from '';
import type { DataGraphqlRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new GraphQLApi();

  const body = {
    // ExecuteGraphQLInput | Request body for data.ExecuteGraphQLInput
    executeGraphQLInput: ...,
  } satisfies DataGraphqlRequest;

  try {
    const data = await api.dataGraphql(body);
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
| **executeGraphQLInput** | [ExecuteGraphQLInput](ExecuteGraphQLInput.md) | Request body for data.ExecuteGraphQLInput | |

### Return type

[**ExecuteGraphQLResponse**](ExecuteGraphQLResponse.md)

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

