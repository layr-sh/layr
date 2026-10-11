# EventsApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**coreEventsGet**](EventsApi.md#coreeventsget) | **GET** /v1/_/core/events/{event_id} | Get individual event record by UUID |
| [**coreEventsList**](EventsApi.md#coreeventslist) | **GET** /v1/_/core/events | Query immutable system and domain audit events |



## coreEventsGet

> Event coreEventsGet(eventId)

Get individual event record by UUID

Retrieves a specific canonical event record by UUID from core.events, including event type, actor principal, transport context (IP, user agent, request ID), and structured payload data. Requires core:event.read scope.

### Example

```ts
import {
  Configuration,
  EventsApi,
} from '';
import type { CoreEventsGetRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventsApi();

  const body = {
    // string
    eventId: eventId_example,
  } satisfies CoreEventsGetRequest;

  try {
    const data = await api.coreEventsGet(body);
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
| **eventId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Event**](Event.md)

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


## coreEventsList

> Array&lt;Event&gt; coreEventsList()

Query immutable system and domain audit events

Queries immutable system and domain events from core.events with multi-field filtering by event type, actor type, date ranges, and pagination limits. Returns full event context, actor, and payload data. Requires core:event.read scope.

### Example

```ts
import {
  Configuration,
  EventsApi,
} from '';
import type { CoreEventsListRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new EventsApi();

  try {
    const data = await api.coreEventsList();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**Array&lt;Event&gt;**](Event.md)

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

