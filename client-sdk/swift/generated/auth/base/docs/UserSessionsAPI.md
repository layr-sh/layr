# UserSessionsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUserSessionsList**](UserSessionsAPI.md#authusersessionslist) | **GET** /v1/auth/user/sessions | List active sessions for current user
[**authUserSessionsRevoke**](UserSessionsAPI.md#authusersessionsrevoke) | **DELETE** /v1/auth/user/sessions/{session_id} | Revoke a specific user session by ID
[**authUserSessionsRevokeOthers**](UserSessionsAPI.md#authusersessionsrevokeothers) | **POST** /v1/auth/user/sessions/revoke-others | Sign out of all other devices and sessions


# **authUserSessionsList**
```swift
    open class func authUserSessionsList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListSessionsResponse
```

List active sessions for current user

Returns active sessions from auth.sessions for the authenticated user, detailing IP address, user agent, creation date, expiry timestamp, and a flag indicating the current device session. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List active sessions for current user
do {
    let response = try await UserSessionsAPI.authUserSessionsList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**ListSessionsResponse**](ListSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserSessionsRevoke**
```swift
    open class func authUserSessionsRevoke(sessionId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Revoke a specific user session by ID

Deletes a specific session record from auth.sessions belonging to the authenticated user, terminating access for that device. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let sessionId = "sessionId_example" // String | 

// Revoke a specific user session by ID
do {
    let response = try await UserSessionsAPI.authUserSessionsRevoke(sessionId: sessionId)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **sessionId** | **String** |  | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUserSessionsRevokeOthers**
```swift
    open class func authUserSessionsRevokeOthers(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> RevokeOtherSessionsResponse
```

Sign out of all other devices and sessions

Revokes all active sessions in auth.sessions belonging to the authenticated user except the current device session, invalidating other refresh tokens immediately. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Sign out of all other devices and sessions
do {
    let response = try await UserSessionsAPI.authUserSessionsRevokeOthers(body: body)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | **JSONValue** | Request body for core.Empty | 

### Return type

[**RevokeOtherSessionsResponse**](RevokeOtherSessionsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

