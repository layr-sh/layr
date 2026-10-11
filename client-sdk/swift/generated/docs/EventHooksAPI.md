# EventHooksAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**coreEventHooksCreate**](EventHooksAPI.md#coreeventhookscreate) | **POST** /v1/_/core/event-hooks | Create an event hook subscription
[**coreEventHooksDelete**](EventHooksAPI.md#coreeventhooksdelete) | **DELETE** /v1/_/core/event-hooks/{event_hook_id} | Delete event hook subscription
[**coreEventHooksGet**](EventHooksAPI.md#coreeventhooksget) | **GET** /v1/_/core/event-hooks/{event_hook_id} | Get event hook subscription details by UUID
[**coreEventHooksList**](EventHooksAPI.md#coreeventhookslist) | **GET** /v1/_/core/event-hooks | List event hook subscriptions
[**coreEventHooksUpdate**](EventHooksAPI.md#coreeventhooksupdate) | **PUT** /v1/_/core/event-hooks/{event_hook_id} | Update event hook subscription


# **coreEventHooksCreate**
```swift
    open class func coreEventHooksCreate(createEventHookInput: CreateEventHookInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> EventHook
```

Create an event hook subscription

Creates an event hook subscription in core.event_hooks for SQL stored procedures (sql driver) or HTTP webhooks (http driver) with HMAC-SHA256 signing secret encryption, wildcard event patterns, timeout guards, and retry policies. Requires core:event-hook.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createEventHookInput = CreateEventHookInput(driver: "driver_example", eventTypes: ["eventTypes_example"], httpTargetUrl: "httpTargetUrl_example", isEnabled: false, maxRetries: 123, name: "name_example", signingSecret: "signingSecret_example", sqlFunctionName: "sqlFunctionName_example", timeoutSeconds: 123) // CreateEventHookInput | Request body for core.CreateEventHookInput

// Create an event hook subscription
do {
    let response = try await EventHooksAPI.coreEventHooksCreate(createEventHookInput: createEventHookInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createEventHookInput** | [**CreateEventHookInput**](CreateEventHookInput.md) | Request body for core.CreateEventHookInput | 

### Return type

[**EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventHooksDelete**
```swift
    open class func coreEventHooksDelete(eventHookId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete event hook subscription

Permanently removes an event hook subscription from core.event_hooks, immediately unbinding event dispatches. Delivery history is preserved or cascaded according to retention rules. Requires core:event-hook.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventHookId = "eventHookId_example" // String | 

// Delete event hook subscription
do {
    let response = try await EventHooksAPI.coreEventHooksDelete(eventHookId: eventHookId)
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

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventHooksGet**
```swift
    open class func coreEventHooksGet(eventHookId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> EventHook
```

Get event hook subscription details by UUID

Retrieves event hook configuration by UUID from core.event_hooks, including driver type, target endpoint, subscribed event patterns, retry limits, and secret configuration indicator. Requires core:event-hook.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventHookId = "eventHookId_example" // String | 

// Get event hook subscription details by UUID
do {
    let response = try await EventHooksAPI.coreEventHooksGet(eventHookId: eventHookId)
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

[**EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventHooksList**
```swift
    open class func coreEventHooksList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [EventHook]
```

List event hook subscriptions

Lists event hook subscriptions from core.event_hooks with driver (sql or http), target function or URL, subscribed event type patterns, retry limits, and enabled status. Requires core:event-hook.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List event hook subscriptions
do {
    let response = try await EventHooksAPI.coreEventHooksList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**[EventHook]**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreEventHooksUpdate**
```swift
    open class func coreEventHooksUpdate(eventHookId: String, updateEventHookInput: UpdateEventHookInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> EventHook
```

Update event hook subscription

Modifies event hook parameters in core.event_hooks including target URL or stored procedure, subscribed event patterns, retry count (0-10), timeout (1-60s), signing secret, or enabled status. Requires core:event-hook.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let eventHookId = "eventHookId_example" // String | 
let updateEventHookInput = UpdateEventHookInput(driver: "driver_example", eventTypes: ["eventTypes_example"], httpTargetUrl: "httpTargetUrl_example", isEnabled: false, maxRetries: 123, name: "name_example", signingSecret: "signingSecret_example", sqlFunctionName: "sqlFunctionName_example", timeoutSeconds: 123) // UpdateEventHookInput | Request body for core.UpdateEventHookInput

// Update event hook subscription
do {
    let response = try await EventHooksAPI.coreEventHooksUpdate(eventHookId: eventHookId, updateEventHookInput: updateEventHookInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **eventHookId** | **String** |  | 
 **updateEventHookInput** | [**UpdateEventHookInput**](UpdateEventHookInput.md) | Request body for core.UpdateEventHookInput | 

### Return type

[**EventHook**](EventHook.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

