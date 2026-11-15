# \EventHooksDeliveriesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_event_hooks_deliveries_list**](EventHooksDeliveriesApi.md#core_event_hooks_deliveries_list) | **GET** /v1/_/core/event-hooks/{event_hook_id}/deliveries | List delivery audit logs for an event hook
[**core_event_hooks_deliveries_retry**](EventHooksDeliveriesApi.md#core_event_hooks_deliveries_retry) | **POST** /v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry | Retry a past event hook delivery attempt



## core_event_hooks_deliveries_list

> Vec<models::EventHookDelivery> core_event_hooks_deliveries_list(event_hook_id)
List delivery audit logs for an event hook

Queries the 100 most recent dispatch attempts for an event hook from core.event_hook_deliveries, detailing event ID, payload, HTTP status code, duration in milliseconds, error messages, and delivery state. Requires core:event-hook.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_hook_id** | **String** |  | [required] |

### Return type

[**Vec<models::EventHookDelivery>**](EventHookDelivery.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_event_hooks_deliveries_retry

> models::EventHookDelivery core_event_hooks_deliveries_retry(event_hook_id, delivery_id, body)
Retry a past event hook delivery attempt

Manually redrives an event hook delivery using the recorded event payload, re-dispatching via the configured driver (SQL procedure or HTTP POST webhook) and recording the new attempt outcome. Requires core:event-hook.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_hook_id** | **String** |  | [required] |
**delivery_id** | **String** |  | [required] |
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::EventHookDelivery**](EventHookDelivery.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

