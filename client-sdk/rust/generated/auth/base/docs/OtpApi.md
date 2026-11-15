# \OtpApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_otp_send**](OtpApi.md#auth_otp_send) | **POST** /v1/auth/otp | Request a one-time password verification code via email or SMS
[**auth_otp_verify**](OtpApi.md#auth_otp_verify) | **POST** /v1/auth/otp/verify | Verify one-time password code and issue authenticated session



## auth_otp_send

> serde_json::Value auth_otp_send(send_otp_input)
Request a one-time password verification code via email or SMS

Generates a single-use 6-digit numeric OTP with configurable TTL and rate limiting, hashes it with Argon2id, stores it in auth.otps, and dispatches it via configured email or SMS provider (Twilio, Vonage, SendGrid, Resend, or SMTP).

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**send_otp_input** | [**SendOtpInput**](SendOtpInput.md) | Request body for auth.SendOTPInput | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_otp_verify

> models::AuthTokenResponse auth_otp_verify(verify_otp_input)
Verify one-time password code and issue authenticated session

Validates the 6-digit OTP code against stored Argon2id hash in auth.otps. On successful verification, marks contact method verified, issues an Ed25519 JWT access token and rotated refresh token, and creates an active session in auth.sessions.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**verify_otp_input** | [**VerifyOtpInput**](VerifyOtpInput.md) | Request body for auth.VerifyOTPInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

