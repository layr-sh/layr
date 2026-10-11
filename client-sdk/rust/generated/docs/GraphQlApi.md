# \GraphQlApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**data_graphql**](GraphQlApi.md#data_graphql) | **POST** /v1/graphql | Execute GraphQL query or mutation



## data_graphql

> models::ExecuteGraphQlResponse data_graphql(execute_graph_ql_input)
Execute GraphQL query or mutation

Dynamic introspection and execution engine over PostgreSQL tables, supporting complex nested selections, arguments, variables, query depth limits (max depth guard), complexity budgets, and native RLS session context injection.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**execute_graph_ql_input** | [**ExecuteGraphQlInput**](ExecuteGraphQlInput.md) | Request body for data.ExecuteGraphQLInput | [required] |

### Return type

[**models::ExecuteGraphQlResponse**](ExecuteGraphQLResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

