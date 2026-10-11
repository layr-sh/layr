# PasskeysSignInAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authPasskeysSignInBegin**](PasskeysSignInAPI.md#authpasskeyssigninbegin) | **POST** /v1/auth/passkeys/sign-in | Begin WebAuthn passkey authentication ceremony
[**authPasskeysSignInVerify**](PasskeysSignInAPI.md#authpasskeyssigninverify) | **POST** /v1/auth/passkeys/sign-in/verify | Verify WebAuthn passkey assertion signature and issue session


# **authPasskeysSignInBegin**
```swift
    open class func authPasskeysSignInBegin(body: JSONValue, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SignInOptions
```

Begin WebAuthn passkey authentication ceremony

Initiates WebAuthn / FIDO2 assertion ceremony, generating a cryptographic challenge and credential request options for the client browser or authenticator hardware.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let body =  // JSONValue | Request body for core.Empty

// Begin WebAuthn passkey authentication ceremony
do {
    let response = try await PasskeysSignInAPI.authPasskeysSignInBegin(body: body)
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

[**SignInOptions**](SignInOptions.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authPasskeysSignInVerify**
```swift
    open class func authPasskeysSignInVerify(verifyPasskeySignInInput: VerifyPasskeySignInInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Verify WebAuthn passkey assertion signature and issue session

Verifies the client's WebAuthn assertion signature against the registered public key and signature counter stored in auth.passkeys. Issues an Ed25519 JWT access token, refresh token, and session record in auth.sessions.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let verifyPasskeySignInInput = VerifyPasskeySignInInput(authenticatorData: "authenticatorData_example", challenge: "challenge_example", clientDataJson: "clientDataJson_example", credentialId: "credentialId_example", signature: "signature_example") // VerifyPasskeySignInInput | Request body for auth.VerifyPasskeySignInInput

// Verify WebAuthn passkey assertion signature and issue session
do {
    let response = try await PasskeysSignInAPI.authPasskeysSignInVerify(verifyPasskeySignInInput: verifyPasskeySignInInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **verifyPasskeySignInInput** | [**VerifyPasskeySignInInput**](VerifyPasskeySignInInput.md) | Request body for auth.VerifyPasskeySignInInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

