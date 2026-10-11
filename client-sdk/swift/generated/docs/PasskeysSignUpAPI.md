# PasskeysSignUpAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authPasskeysSignUpBegin**](PasskeysSignUpAPI.md#authpasskeyssignupbegin) | **POST** /v1/auth/passkeys/sign-up | Begin WebAuthn passkey registration ceremony
[**authPasskeysSignUpVerify**](PasskeysSignUpAPI.md#authpasskeyssignupverify) | **POST** /v1/auth/passkeys/sign-up/verify | Verify WebAuthn passkey registration attestation and store credential


# **authPasskeysSignUpBegin**
```swift
    open class func authPasskeysSignUpBegin(beginPasskeySignUpInput: BeginPasskeySignUpInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> SignUpOptions
```

Begin WebAuthn passkey registration ceremony

Initiates WebAuthn / FIDO2 credential creation ceremony, generating an attestation challenge, relying party metadata, and user entity options for hardware or platform passkey enrollment.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let beginPasskeySignUpInput = BeginPasskeySignUpInput(userId: "userId_example", userName: "userName_example") // BeginPasskeySignUpInput | Request body for auth.BeginPasskeySignUpInput

// Begin WebAuthn passkey registration ceremony
do {
    let response = try await PasskeysSignUpAPI.authPasskeysSignUpBegin(beginPasskeySignUpInput: beginPasskeySignUpInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **beginPasskeySignUpInput** | [**BeginPasskeySignUpInput**](BeginPasskeySignUpInput.md) | Request body for auth.BeginPasskeySignUpInput | 

### Return type

[**SignUpOptions**](SignUpOptions.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authPasskeysSignUpVerify**
```swift
    open class func authPasskeysSignUpVerify(verifyPasskeySignUpInput: VerifyPasskeySignUpInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Verify WebAuthn passkey registration attestation and store credential

Validates client attestation response and public key, stores credential record in auth.passkeys, creates user identity, and issues an authenticated session with Ed25519 JWT tokens.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let verifyPasskeySignUpInput = VerifyPasskeySignUpInput(challenge: "challenge_example", credentialId: "credentialId_example", friendlyName: "friendlyName_example", publicKey: "publicKey_example", transports: ["transports_example"]) // VerifyPasskeySignUpInput | Request body for auth.VerifyPasskeySignUpInput

// Verify WebAuthn passkey registration attestation and store credential
do {
    let response = try await PasskeysSignUpAPI.authPasskeysSignUpVerify(verifyPasskeySignUpInput: verifyPasskeySignUpInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **verifyPasskeySignUpInput** | [**VerifyPasskeySignUpInput**](VerifyPasskeySignUpInput.md) | Request body for auth.VerifyPasskeySignUpInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

