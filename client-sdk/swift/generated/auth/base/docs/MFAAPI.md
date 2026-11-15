# MFAAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authMfaChallenge**](MFAAPI.md#authmfachallenge) | **POST** /v1/auth/mfa/challenge | Verify MFA challenge ticket with TOTP passcode
[**authMfaDisable**](MFAAPI.md#authmfadisable) | **DELETE** /v1/auth/mfa | Disable multi-factor authentication on user account
[**authMfaSetup**](MFAAPI.md#authmfasetup) | **POST** /v1/auth/mfa | Generate TOTP secret and setup URI for authenticator enrollment
[**authMfaVerify**](MFAAPI.md#authmfaverify) | **POST** /v1/auth/mfa/verify | Verify initial TOTP code to complete MFA enrollment


# **authMfaChallenge**
```swift
    open class func authMfaChallenge(challengeMFAInput: ChallengeMFAInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Verify MFA challenge ticket with TOTP passcode

Verifies a 6-digit TOTP passcode against the user's enrolled MFA secret using a short-lived challenge ticket (5-minute TTL) issued during password sign-in. On verification, issues an Ed25519 JWT access token, rotated refresh token, and creates a session record in auth.sessions.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let challengeMFAInput = ChallengeMFAInput(code: "code_example", mfaTicket: "mfaTicket_example") // ChallengeMFAInput | Request body for auth.ChallengeMFAInput

// Verify MFA challenge ticket with TOTP passcode
do {
    let response = try await MFAAPI.authMfaChallenge(challengeMFAInput: challengeMFAInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **challengeMFAInput** | [**ChallengeMFAInput**](ChallengeMFAInput.md) | Request body for auth.ChallengeMFAInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authMfaDisable**
```swift
    open class func authMfaDisable(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Disable multi-factor authentication on user account

Disables TOTP multi-factor authentication on auth.users for the authenticated user, clears the encrypted MFA secret, and publishes auth.mfa.disabled event. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Disable multi-factor authentication on user account
do {
    let response = try await MFAAPI.authMfaDisable()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authMfaSetup**
```swift
    open class func authMfaSetup(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SetupMFAResponse
```

Generate TOTP secret and setup URI for authenticator enrollment

Generates a cryptographic RFC 6238 TOTP secret, stores it temporarily encrypted in the session cache, and returns an otpauth:// URI and QR code payload compatible with Google Authenticator, 1Password, or Authy. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Generate TOTP secret and setup URI for authenticator enrollment
do {
    let response = try await MFAAPI.authMfaSetup(body: body)
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

[**SetupMFAResponse**](SetupMFAResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authMfaVerify**
```swift
    open class func authMfaVerify(verifyMFAInput: VerifyMFAInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Verify initial TOTP code to complete MFA enrollment

Validates the initial 6-digit TOTP code against the pending setup secret, sets mfa_enabled to true on auth.users, stores the AES-encrypted secret permanently, and publishes auth.mfa.enabled event. Requires authenticated user session.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let verifyMFAInput = VerifyMFAInput(code: "code_example") // VerifyMFAInput | Request body for auth.VerifyMFAInput

// Verify initial TOTP code to complete MFA enrollment
do {
    let response = try await MFAAPI.authMfaVerify(verifyMFAInput: verifyMFAInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **verifyMFAInput** | [**VerifyMFAInput**](VerifyMFAInput.md) | Request body for auth.VerifyMFAInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

