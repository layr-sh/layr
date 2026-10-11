# UsersSessionsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUsersSessionsList**](UsersSessionsAPI.md#authuserssessionslist) | **GET** /v1/_/auth/users/{user_id}/sessions | List active sessions for an application user
[**authUsersSessionsRevoke**](UsersSessionsAPI.md#authuserssessionsrevoke) | **POST** /v1/_/auth/users/{user_id}/sessions/revoke | Revoke all active sessions for an application user
[**consoleUsersSessionsDelete**](UsersSessionsAPI.md#consoleuserssessionsdelete) | **DELETE** /v1/_/console/users/{user_id}/sessions/{session_id} | Revoke a specific console user session
[**consoleUsersSessionsList**](UsersSessionsAPI.md#consoleuserssessionslist) | **GET** /v1/_/console/users/{user_id}/sessions | List active sessions for a console user
[**consoleUsersSessionsRevoke**](UsersSessionsAPI.md#consoleuserssessionsrevoke) | **POST** /v1/_/console/users/{user_id}/sessions/revoke | Revoke all active sessions for a console user


# **authUsersSessionsList**
```swift
    open class func authUsersSessionsList(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListUserSessionsResponse
```

List active sessions for an application user

Returns all active sessions from auth.sessions for the specified user ID, detailing client IP, user agent, expiration time, and creation timestamp. Requires auth:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// List active sessions for an application user
do {
    let response = try await UsersSessionsAPI.authUsersSessionsList(userId: userId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 

### Return type

[**ListUserSessionsResponse**](ListUserSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersSessionsRevoke**
```swift
    open class func authUsersSessionsRevoke(userId: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke all active sessions for an application user

Deletes all active sessions from auth.sessions for the specified user ID, invalidating all outstanding refresh tokens and forcing re-authentication. Requires auth:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let body =  // JSONValue | Request body for core.Empty

// Revoke all active sessions for an application user
do {
    let response = try await UsersSessionsAPI.authUsersSessionsRevoke(userId: userId, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersSessionsDelete**
```swift
    open class func consoleUsersSessionsDelete(userId: String, sessionId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke a specific console user session

Terminates a single session by session UUID from console.sessions for the specified console user. Emits console.user.session.revoked event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let sessionId = "sessionId_example" // String | 

// Revoke a specific console user session
do {
    let response = try await UsersSessionsAPI.consoleUsersSessionsDelete(userId: userId, sessionId: sessionId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **sessionId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersSessionsList**
```swift
    open class func consoleUsersSessionsList(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [Session]
```

List active sessions for a console user

Returns all active sessions from console.sessions for the specified console user, including client IP address, user-agent string, creation timestamp, and expiration time. Requires console:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// List active sessions for a console user
do {
    let response = try await UsersSessionsAPI.consoleUsersSessionsList(userId: userId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 

### Return type

[**[Session]**](Session.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersSessionsRevoke**
```swift
    open class func consoleUsersSessionsRevoke(userId: String, body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke all active sessions for a console user

Terminates all active sessions in console.sessions for the specified console user, immediately invalidating cookie tokens. Emits console.user.sessions.revoked_all event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let body =  // JSONValue | Request body for core.Empty

// Revoke all active sessions for a console user
do {
    let response = try await UsersSessionsAPI.consoleUsersSessionsRevoke(userId: userId, body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

