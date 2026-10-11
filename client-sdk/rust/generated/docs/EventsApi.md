# \EventsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_events_get**](EventsApi.md#core_events_get) | **GET** /v1/_/core/events/{event_id} | Get individual event record by UUID
[**core_events_list**](EventsApi.md#core_events_list) | **GET** /v1/_/core/events | Query immutable system and domain audit events



## core_events_get

> models::Event core_events_get(event_id)
Get individual event record by UUID

Retrieves a specific canonical event record by UUID from core.events, including event type, actor principal, transport context (IP, user agent, request ID), and structured payload data. Requires core:event.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_id** | **String** |  | [required] |

### Return type

[**models::Event**](Event.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_events_list

> models::ListEventsResponse core_events_list(limit, offset, r#type, actor_type, actor_id, resource_type, resource_id)
Query immutable system and domain audit events

Queries immutable system and domain events from core.events with multi-field filtering by event type, actor type, date ranges, and pagination limits. Returns full event context, actor, and payload data. Requires core:event.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**limit** | Option<**i32**> | Maximum number of items to return (default 50, maximum 1000) |  |[default to 50]
**offset** | Option<**i32**> | Number of items to skip before returning results (default 0) |  |[default to 0]
**r#type** | Option<**String**> | Filter by specific event type name |  |
**actor_type** | Option<**String**> | Filter by actor principal type (user, service_account, system) |  |
**actor_id** | Option<**String**> | Filter by actor principal UUID |  |
**resource_type** | Option<**String**> | Filter by target resource entity type |  |
**resource_id** | Option<**String**> | Filter by target resource unique identifier |  |

### Return type

[**models::ListEventsResponse**](ListEventsResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

