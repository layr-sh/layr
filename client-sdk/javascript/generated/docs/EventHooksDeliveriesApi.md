# EventHooksDeliveriesApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreEventHooksDeliveriesList**](EventHooksDeliveriesApi.md#coreeventhooksdeliverieslist) | **GET** /v1/_/core/event-hooks/{event_hook_id}/deliveries | List delivery audit logs for an event hook |
| [**coreEventHooksDeliveriesRetry**](EventHooksDeliveriesApi.md#coreeventhooksdeliveriesretry) | **POST** /v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry | Retry a past event hook delivery attempt |



## coreEventHooksDeliveriesList

> Array&lt;EventHookDelivery&gt; coreEventHooksDeliveriesList(eventHookId)

List delivery audit logs for an event hook

Queries the 100 most recent dispatch attempts for an event hook from core.event_hook_deliveries, detailing event ID, payload, HTTP status code, duration in milliseconds, error messages, and delivery state. Requires core:event-hook.read scope.

### Example

```ts
import {
  Configuration,
  EventHooksDeliveriesApi,
} from '';
import type { CoreEventHooksDeliveriesListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksDeliveriesApi();

  const body = {
    // string
    eventHookId: eventHookId_example,
  } satisfies CoreEventHooksDeliveriesListRequest;

  try {
    const data = await api.coreEventHooksDeliveriesList(body);
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

[**Array&lt;EventHookDelivery&gt;**](EventHookDelivery.md)

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


## coreEventHooksDeliveriesRetry

> EventHookDelivery coreEventHooksDeliveriesRetry(eventHookId, deliveryId, body)

Retry a past event hook delivery attempt

Manually redrives an event hook delivery using the recorded event payload, re-dispatching via the configured driver (SQL procedure or HTTP POST webhook) and recording the new attempt outcome. Requires core:event-hook.write scope.

### Example

```ts
import {
  Configuration,
  EventHooksDeliveriesApi,
} from '';
import type { CoreEventHooksDeliveriesRetryRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventHooksDeliveriesApi();

  const body = {
    // string
    eventHookId: eventHookId_example,
    // string
    deliveryId: deliveryId_example,
    // any | Request body for core.Empty
    body: ...,
  } satisfies CoreEventHooksDeliveriesRetryRequest;

  try {
    const data = await api.coreEventHooksDeliveriesRetry(body);
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
| **deliveryId** | `string` |  | [Defaults to `undefined`] |
| **body** | `any` | Request body for core.Empty | |

### Return type

[**EventHookDelivery**](EventHookDelivery.md)

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

