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
[**consoleUsersCreate**](UsersAPI.md#consoleuserscreate) | **POST** /v1/_/console/users | Create console user account
[**consoleUsersDelete**](UsersAPI.md#consoleusersdelete) | **DELETE** /v1/_/console/users/{user_id} | Delete console user account and cascade sessions
[**consoleUsersGet**](UsersAPI.md#consoleusersget) | **GET** /v1/_/console/users/{user_id} | Get console user account details by UUID
[**consoleUsersList**](UsersAPI.md#consoleuserslist) | **GET** /v1/_/console/users | List console user accounts
[**consoleUsersResetPassword**](UsersAPI.md#consoleusersresetpassword) | **POST** /v1/_/console/users/{user_id}/password | Reset console user password and revoke sessions
[**consoleUsersUpdate**](UsersAPI.md#consoleusersupdate) | **PUT** /v1/_/console/users/{user_id} | Update console user account properties


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
    open class func authUsersList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> ListUsersResponse
```

List registered application users with filtering and pagination

Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List registered application users with filtering and pagination
do {
    let response = try await UsersAPI.authUsersList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

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

# **consoleUsersCreate**
```swift
    open class func consoleUsersCreate(createConsoleUserInput: CreateConsoleUserInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Create console user account

Creates a new console user account in console.users with email and Argon2id password hash (minimum 8 characters). Publishes console.user.created event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let createConsoleUserInput = CreateConsoleUserInput(email: "email_example", password: "password_example") // CreateConsoleUserInput | Request body for console.CreateConsoleUserInput

// Create console user account
do {
    let response = try await UsersAPI.consoleUsersCreate(createConsoleUserInput: createConsoleUserInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **createConsoleUserInput** | [**CreateConsoleUserInput**](CreateConsoleUserInput.md) | Request body for console.CreateConsoleUserInput | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersDelete**
```swift
    open class func consoleUsersDelete(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Delete console user account and cascade sessions

Permanently deletes console user account from console.users, automatically cascading revocation of all active sessions in console.sessions. Emits console.user.deleted event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// Delete console user account and cascade sessions
do {
    let response = try await UsersAPI.consoleUsersDelete(userId: userId)
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

# **consoleUsersGet**
```swift
    open class func consoleUsersGet(userId: String, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Get console user account details by UUID

Retrieves a single console user account from console.users by UUID, including email, enabled status, MFA enrollment flag, and timestamps. Requires console:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 

// Get console user account details by UUID
do {
    let response = try await UsersAPI.consoleUsersGet(userId: userId)
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

# **consoleUsersList**
```swift
    open class func consoleUsersList(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> [User]
```

List console user accounts

Returns all registered console user accounts from console.users with email, active status, MFA enrollment status, last login timestamp, and creation date. Requires console:user.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// List console user accounts
do {
    let response = try await UsersAPI.consoleUsersList()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**[User]**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersResetPassword**
```swift
    open class func consoleUsersResetPassword(userId: String, resetConsoleUserPasswordInput: ResetConsoleUserPasswordInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue
```

Reset console user password and revoke sessions

Updates password hash in console.users using Argon2id key derivation and revokes all active sessions in console.sessions, forcing re-login across all devices. Emits console.user.password_reset event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let resetConsoleUserPasswordInput = ResetConsoleUserPasswordInput(newPassword: "newPassword_example") // ResetConsoleUserPasswordInput | Request body for console.ResetConsoleUserPasswordInput

// Reset console user password and revoke sessions
do {
    let response = try await UsersAPI.consoleUsersResetPassword(userId: userId, resetConsoleUserPasswordInput: resetConsoleUserPasswordInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **resetConsoleUserPasswordInput** | [**ResetConsoleUserPasswordInput**](ResetConsoleUserPasswordInput.md) | Request body for console.ResetConsoleUserPasswordInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleUsersUpdate**
```swift
    open class func consoleUsersUpdate(userId: String, updateConsoleUserInput: UpdateConsoleUserInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> User
```

Update console user account properties

Modifies email or enabled status of a console user in console.users. If account is disabled, immediately revokes all active sessions in console.sessions. Emits console.user.updated event. Requires console:user.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let userId = "userId_example" // String | 
let updateConsoleUserInput = UpdateConsoleUserInput(email: "email_example", isEnabled: false) // UpdateConsoleUserInput | Request body for console.UpdateConsoleUserInput

// Update console user account properties
do {
    let response = try await UsersAPI.consoleUsersUpdate(userId: userId, updateConsoleUserInput: updateConsoleUserInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **userId** | **String** |  | 
 **updateConsoleUserInput** | [**UpdateConsoleUserInput**](UpdateConsoleUserInput.md) | Request body for console.UpdateConsoleUserInput | 

### Return type

[**User**](User.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

