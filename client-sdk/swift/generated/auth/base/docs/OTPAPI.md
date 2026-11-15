# OTPAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authOtpSend**](OTPAPI.md#authotpsend) | **POST** /v1/auth/otp | Request a one-time password verification code via email or SMS
[**authOtpVerify**](OTPAPI.md#authotpverify) | **POST** /v1/auth/otp/verify | Verify one-time password code and issue authenticated session


# **authOtpSend**
```swift
    open class func authOtpSend(sendOTPInput: SendOTPInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> JSONValue?
```

Request a one-time password verification code via email or SMS

Generates a single-use 6-digit numeric OTP with configurable TTL and rate limiting, hashes it with Argon2id, stores it in auth.otps, and dispatches it via configured email or SMS provider (Twilio, Vonage, SendGrid, Resend, or SMTP).

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let sendOTPInput = SendOTPInput(captchaToken: "captchaToken_example", purpose: "purpose_example", recipient: "recipient_example") // SendOTPInput | Request body for auth.SendOTPInput

// Request a one-time password verification code via email or SMS
do {
    let response = try await OTPAPI.authOtpSend(sendOTPInput: sendOTPInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **sendOTPInput** | [**SendOTPInput**](SendOTPInput.md) | Request body for auth.SendOTPInput | 

### Return type

**JSONValue**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **authOtpVerify**
```swift
    open class func authOtpVerify(verifyOTPInput: VerifyOTPInput, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> AuthTokenResponse
```

Verify one-time password code and issue authenticated session

Validates the 6-digit OTP code against stored Argon2id hash in auth.otps. On successful verification, marks contact method verified, issues an Ed25519 JWT access token and rotated refresh token, and creates an active session in auth.sessions.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let verifyOTPInput = VerifyOTPInput(captchaToken: "captchaToken_example", code: "code_example", purpose: "purpose_example", recipient: "recipient_example") // VerifyOTPInput | Request body for auth.VerifyOTPInput

// Verify one-time password code and issue authenticated session
do {
    let response = try await OTPAPI.authOtpVerify(verifyOTPInput: verifyOTPInput)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **verifyOTPInput** | [**VerifyOTPInput**](VerifyOTPInput.md) | Request body for auth.VerifyOTPInput | 

### Return type

[**AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

