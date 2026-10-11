# \EventHooksApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**core_event_hooks_create**](EventHooksApi.md#core_event_hooks_create) | **POST** /v1/_/core/event-hooks | Create an event hook subscription
[**core_event_hooks_delete**](EventHooksApi.md#core_event_hooks_delete) | **DELETE** /v1/_/core/event-hooks/{event_hook_id} | Delete event hook subscription
[**core_event_hooks_get**](EventHooksApi.md#core_event_hooks_get) | **GET** /v1/_/core/event-hooks/{event_hook_id} | Get event hook subscription details by UUID
[**core_event_hooks_list**](EventHooksApi.md#core_event_hooks_list) | **GET** /v1/_/core/event-hooks | List event hook subscriptions
[**core_event_hooks_update**](EventHooksApi.md#core_event_hooks_update) | **PUT** /v1/_/core/event-hooks/{event_hook_id} | Update event hook subscription



## core_event_hooks_create

> models::EventHook core_event_hooks_create(create_event_hook_input)
Create an event hook subscription

Creates an event hook subscription in core.event_hooks for SQL stored procedures (sql driver) or HTTP webhooks (http driver) with HMAC-SHA256 signing secret encryption, wildcard event patterns, timeout guards, and retry policies. Requires core:event-hook.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**create_event_hook_input** | [**CreateEventHookInput**](CreateEventHookInput.md) | Request body for core.CreateEventHookInput | [required] |

### Return type

[**models::EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_event_hooks_delete

> serde_json::Value core_event_hooks_delete(event_hook_id)
Delete event hook subscription

Permanently removes an event hook subscription from core.event_hooks, immediately unbinding event dispatches. Delivery history is preserved or cascaded according to retention rules. Requires core:event-hook.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_hook_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_event_hooks_get

> models::EventHook core_event_hooks_get(event_hook_id)
Get event hook subscription details by UUID

Retrieves event hook configuration by UUID from core.event_hooks, including driver type, target endpoint, subscribed event patterns, retry limits, and secret configuration indicator. Requires core:event-hook.read scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_hook_id** | **String** |  | [required] |

### Return type

[**models::EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_event_hooks_list

> Vec<models::EventHook> core_event_hooks_list()
List event hook subscriptions

Lists event hook subscriptions from core.event_hooks with driver (sql or http), target function or URL, subscribed event type patterns, retry limits, and enabled status. Requires core:event-hook.read scope.

### Parameters

This endpoint does not need any parameter.

### Return type

[**Vec<models::EventHook>**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## core_event_hooks_update

> models::EventHook core_event_hooks_update(event_hook_id, update_event_hook_input)
Update event hook subscription

Modifies event hook parameters in core.event_hooks including target URL or stored procedure, subscribed event patterns, retry count (0-10), timeout (1-60s), signing secret, or enabled status. Requires core:event-hook.write scope.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**event_hook_id** | **String** |  | [required] |
**update_event_hook_input** | [**UpdateEventHookInput**](UpdateEventHookInput.md) | Request body for core.UpdateEventHookInput | [required] |

### Return type

[**models::EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

