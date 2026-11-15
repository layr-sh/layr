# EventHooksApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreEventHooksCreate**](EventHooksApi.md#coreeventhookscreate) | **POST** /v1/_/core/event-hooks | Create an event hook subscription |
| [**coreEventHooksDelete**](EventHooksApi.md#coreeventhooksdelete) | **DELETE** /v1/_/core/event-hooks/{event_hook_id} | Delete event hook subscription |
| [**coreEventHooksGet**](EventHooksApi.md#coreeventhooksget) | **GET** /v1/_/core/event-hooks/{event_hook_id} | Get event hook subscription details by UUID |
| [**coreEventHooksList**](EventHooksApi.md#coreeventhookslist) | **GET** /v1/_/core/event-hooks | List event hook subscriptions |
| [**coreEventHooksUpdate**](EventHooksApi.md#coreeventhooksupdate) | **PUT** /v1/_/core/event-hooks/{event_hook_id} | Update event hook subscription |



## coreEventHooksCreate

> EventHook coreEventHooksCreate(createEventHookInput)

Create an event hook subscription

Creates an event hook subscription in core.event_hooks for SQL stored procedures (sql driver) or HTTP webhooks (http driver) with HMAC-SHA256 signing secret encryption, wildcard event patterns, timeout guards, and retry policies. Requires core:event-hook.write scope.

### Example

```ts
import {
  Configuration,
  EventHooksApi,
} from '';
import type { CoreEventHooksCreateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksApi();

  const body = {
    // CreateEventHookInput | Request body for core.CreateEventHookInput
    createEventHookInput: ...,
  } satisfies CoreEventHooksCreateRequest;

  try {
    const data = await api.coreEventHooksCreate(body);
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
| **createEventHookInput** | [CreateEventHookInput](CreateEventHookInput.md) | Request body for core.CreateEventHookInput | |

### Return type

[**EventHook**](EventHook.md)

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


## coreEventHooksDelete

> any coreEventHooksDelete(eventHookId)

Delete event hook subscription

Permanently removes an event hook subscription from core.event_hooks, immediately unbinding event dispatches. Delivery history is preserved or cascaded according to retention rules. Requires core:event-hook.write scope.

### Example

```ts
import {
  Configuration,
  EventHooksApi,
} from '';
import type { CoreEventHooksDeleteRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksApi();

  const body = {
    // string
    eventHookId: eventHookId_example,
  } satisfies CoreEventHooksDeleteRequest;

  try {
    const data = await api.coreEventHooksDelete(body);
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
| **eventHookId** | `string` |  | [Defaults to `undefined`] |

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
| **204** | Event hook deleted |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## coreEventHooksGet

> EventHook coreEventHooksGet(eventHookId)

Get event hook subscription details by UUID

Retrieves event hook configuration by UUID from core.event_hooks, including driver type, target endpoint, subscribed event patterns, retry limits, and secret configuration indicator. Requires core:event-hook.read scope.

### Example

```ts
import {
  Configuration,
  EventHooksApi,
} from '';
import type { CoreEventHooksGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksApi();

  const body = {
    // string
    eventHookId: eventHookId_example,
  } satisfies CoreEventHooksGetRequest;

  try {
    const data = await api.coreEventHooksGet(body);
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
| **eventHookId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**EventHook**](EventHook.md)

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


## coreEventHooksList

> Array&lt;EventHook&gt; coreEventHooksList(driver, isEnabled)

List event hook subscriptions

Lists event hook subscriptions from core.event_hooks with driver (sql or http), target function or URL, subscribed event type patterns, retry limits, and enabled status. Requires core:event-hook.read scope.

### Example

```ts
import {
  Configuration,
  EventHooksApi,
} from '';
import type { CoreEventHooksListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksApi();

  const body = {
    // string | Filter event hooks by delivery driver (\'sql\' or \'http\') (optional)
    driver: driver_example,
    // boolean | Filter event hooks by active enabled status (optional)
    isEnabled: true,
  } satisfies CoreEventHooksListRequest;

  try {
    const data = await api.coreEventHooksList(body);
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
| **driver** | `string` | Filter event hooks by delivery driver (\&#39;sql\&#39; or \&#39;http\&#39;) | [Optional] [Defaults to `undefined`] |
| **isEnabled** | `boolean` | Filter event hooks by active enabled status | [Optional] [Defaults to `undefined`] |

### Return type

[**Array&lt;EventHook&gt;**](EventHook.md)

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


## coreEventHooksUpdate

> EventHook coreEventHooksUpdate(eventHookId, updateEventHookInput)

Update event hook subscription

Modifies event hook parameters in core.event_hooks including target URL or stored procedure, subscribed event patterns, retry count (0-10), timeout (1-60s), signing secret, or enabled status. Requires core:event-hook.write scope.

### Example

```ts
import {
  Configuration,
  EventHooksApi,
} from '';
import type { CoreEventHooksUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksApi();

  const body = {
    // string
    eventHookId: eventHookId_example,
    // UpdateEventHookInput | Request body for core.UpdateEventHookInput
    updateEventHookInput: ...,
  } satisfies CoreEventHooksUpdateRequest;

  try {
    const data = await api.coreEventHooksUpdate(body);
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
| **eventHookId** | `string` |  | [Defaults to `undefined`] |
| **updateEventHookInput** | [UpdateEventHookInput](UpdateEventHookInput.md) | Request body for core.UpdateEventHookInput | |

### Return type

[**EventHook**](EventHook.md)

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

