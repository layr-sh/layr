# UsersAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authUsersCreate**](UsersAPI.md#authuserscreate) | **POST** /v1/_/auth/users | Create an application user account via control plane
[**authUsersDelete**](UsersAPI.md#authusersdelete) | **DELETE** /v1/_/auth/users/{user_id} | Permanently delete an application user and cascade sessions
[**authUsersGet**](UsersAPI.md#authusersget) | **GET** /v1/_/auth/users/{user_id} | Get detailed application user record by UUID
[**authUsersList**](UsersAPI.md#authuserslist) | **GET** /v1/_/auth/users | List registered application users with filtering and pagination
[**authUsersLock**](UsersAPI.md#authuserslock) | **POST** /v1/_/auth/users/{user_id}/lock | Lock application user account and revoke active sessions
[**authUsersUnlock**](UsersAPI.md#authusersunlock) | **DELETE** /v1/_/auth/users/{user_id}/lock | Unlock application user account and restore login access


# **authUsersCreate**
```swift
    open class func authUsersCreate(createUserInput: CreateUserInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Create an application user account via control plane

Directly provisions a new user record in auth.users with specified email, normalized phone, Argon2id password hash, role, custom properties, and email/phone verification flags. Emits auth.user.created event. Requires auth:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createUserInput = CreateUserInput(email: "email_example", emailVerified: false, password: "password_example", phone: "phone_example", phoneVerified: false, properties: "TODO", role: "role_example") // CreateUserInput | Request body for auth.CreateUserInput

// Create an application user account via control plane
do {
    let response = try await UsersAPI.authUsersCreate(createUserInput: createUserInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createUserInput** | [**CreateUserInput**](CreateUserInput.md) | Request body for auth.CreateUserInput | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersDelete**
```swift
    open class func authUsersDelete(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Permanently delete an application user and cascade sessions

Permanently deletes application user from auth.users, cascading deletion to all active sessions, linked identities, and passkeys. Emits auth.user.deleted event. Requires auth:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// Permanently delete an application user and cascade sessions
do {
    let response = try await UsersAPI.authUsersDelete(userId: userId)
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

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersGet**
```swift
    open class func authUsersGet(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Get detailed application user record by UUID

Retrieves complete user record from auth.users by UUID, including verification timestamps, role, custom properties JSON, MFA status, lockout metadata, and linked authentication identities. Requires auth:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// Get detailed application user record by UUID
do {
    let response = try await UsersAPI.authUsersGet(userId: userId)
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

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersList**
```swift
    open class func authUsersList(limit: Int? = nil, offset: Int? = nil, role: String? = nil, search: String? = nil, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListUsersResponse
```

List registered application users with filtering and pagination

Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let limit = 987 // Int | Maximum number of items to return (default 50, maximum 1000) (optional) (default to 50)
let offset = 987 // Int | Number of items to skip before returning results (default 0) (optional) (default to 0)
let role = "role_example" // String | Filter users by role name (optional)
let search = "search_example" // String | Case-insensitive substring search matching email or phone number (optional)

// List registered application users with filtering and pagination
do {
    let response = try await UsersAPI.authUsersList(limit: limit, offset: offset, role: role, search: search)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **limit** | **Int** | Maximum number of items to return (default 50, maximum 1000) | [optional] [default to 50]
 **offset** | **Int** | Number of items to skip before returning results (default 0) | [optional] [default to 0]
 **role** | **String** | Filter users by role name | [optional] 
 **search** | **String** | Case-insensitive substring search matching email or phone number | [optional] 

### Return type

[**ListUsersResponse**](ListUsersResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersLock**
```swift
    open class func authUsersLock(userId: String, lockUserInput: LockUserInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Lock application user account and revoke active sessions

Sets locked_until timestamp on auth.users to prevent further authentication, and immediately deletes all active sessions in auth.sessions. Emits auth.user.locked event. Requires auth:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let lockUserInput = LockUserInput(lockedUntil: Date()) // LockUserInput | Request body for auth.LockUserInput

// Lock application user account and revoke active sessions
do {
    let response = try await UsersAPI.authUsersLock(userId: userId, lockUserInput: lockUserInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **lockUserInput** | [**LockUserInput**](LockUserInput.md) | Request body for auth.LockUserInput | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authUsersUnlock**
```swift
    open class func authUsersUnlock(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Unlock application user account and restore login access

Clears locked_until timestamp in auth.users, restoring authentication capabilities for a user previously locked by control plane action or brute-force protection. Requires auth:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// Unlock application user account and restore login access
do {
    let response = try await UsersAPI.authUsersUnlock(userId: userId)
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

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

