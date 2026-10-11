# EventHooksDeliveriesAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreEventHooksDeliveriesList**](EventHooksDeliveriesAPI.md#coreeventhooksdeliverieslist) | **GET** /v1/_/core/event-hooks/{event_hook_id}/deliveries | List delivery audit logs for an event hook
[**coreEventHooksDeliveriesRetry**](EventHooksDeliveriesAPI.md#coreeventhooksdeliveriesretry) | **POST** /v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry | Retry a past event hook delivery attempt


# **coreEventHooksDeliveriesList**
```swift
    open class func coreEventHooksDeliveriesList(eventHookId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [EventHookDelivery]
```

List delivery audit logs for an event hook

Queries the 100 most recent dispatch attempts for an event hook from core.event_hook_deliveries, detailing event ID, payload, HTTP status code, duration in milliseconds, error messages, and delivery state. Requires core:event-hook.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventHookId = "eventHookId_example" // String | 

// List delivery audit logs for an event hook
do {
    let response = try await EventHooksDeliveriesAPI.coreEventHooksDeliveriesList(eventHookId: eventHookId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **eventHookId** | **String** |  | 

### Return type

[**[EventHookDelivery]**](EventHookDelivery.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventHooksDeliveriesRetry**
```swift
    open class func coreEventHooksDeliveriesRetry(eventHookId: String, deliveryId: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> EventHookDelivery
```

Retry a past event hook delivery attempt

Manually redrives an event hook delivery using the recorded event payload, re-dispatching via the configured driver (SQL procedure or HTTP POST webhook) and recording the new attempt outcome. Requires core:event-hook.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventHookId = "eventHookId_example" // String | 
let deliveryId = "deliveryId_example" // String | 
let body =  // JSONValue | Request body for core.Empty

// Retry a past event hook delivery attempt
do {
    let response = try await EventHooksDeliveriesAPI.coreEventHooksDeliveriesRetry(eventHookId: eventHookId, deliveryId: deliveryId, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **eventHookId** | **String** |  | 
 **deliveryId** | **String** |  | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**EventHookDelivery**](EventHookDelivery.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

