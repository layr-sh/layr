# EventsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreEventsGet**](EventsAPI.md#coreeventsget) | **GET** /v1/_/core/events/{event_id} | Get individual event record by UUID
[**coreEventsList**](EventsAPI.md#coreeventslist) | **GET** /v1/_/core/events | Query immutable system and domain audit events


# **coreEventsGet**
```swift
    open class func coreEventsGet(eventId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Event
```

Get individual event record by UUID

Retrieves a specific canonical event record by UUID from core.events, including event type, actor principal, transport context (IP, user agent, request ID), and structured payload data. Requires core:event.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventId = "eventId_example" // String | 

// Get individual event record by UUID
do {
    let response = try await EventsAPI.coreEventsGet(eventId: eventId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **eventId** | **String** |  | 

### Return type

[**Event**](Event.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventsList**
```swift
    open class func coreEventsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [Event]
```

Query immutable system and domain audit events

Queries immutable system and domain events from core.events with multi-field filtering by event type, actor type, date ranges, and pagination limits. Returns full event context, actor, and payload data. Requires core:event.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Query immutable system and domain audit events
do {
    let response = try await EventsAPI.coreEventsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**[Event]**](Event.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

