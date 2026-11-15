# \MfaApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_mfa_challenge**](MfaApi.md#auth_mfa_challenge) | **POST** /v1/auth/mfa/challenge | Verify MFA challenge ticket with TOTP passcode
[**auth_mfa_disable**](MfaApi.md#auth_mfa_disable) | **DELETE** /v1/auth/mfa | Disable multi-factor authentication on user account
[**auth_mfa_setup**](MfaApi.md#auth_mfa_setup) | **POST** /v1/auth/mfa | Generate TOTP secret and setup URI for authenticator enrollment
[**auth_mfa_verify**](MfaApi.md#auth_mfa_verify) | **POST** /v1/auth/mfa/verify | Verify initial TOTP code to complete MFA enrollment



## auth_mfa_challenge

> models::AuthTokenResponse auth_mfa_challenge(challenge_mfa_input)
Verify MFA challenge ticket with TOTP passcode

Verifies a 6-digit TOTP passcode against the user's enrolled MFA secret using a short-lived challenge ticket (5-minute TTL) issued during password sign-in. On verification, issues an Ed25519 JWT access token, rotated refresh token, and creates a session record in auth.sessions.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**challenge_mfa_input** | [**ChallengeMfaInput**](ChallengeMfaInput.md) | Request body for auth.ChallengeMFAInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_mfa_disable

> serde_json::Value auth_mfa_disable()
Disable multi-factor authentication on user account

Disables TOTP multi-factor authentication on auth.users for the authenticated user, clears the encrypted MFA secret, and publishes auth.mfa.disabled event. Requires authenticated user session.

### Parameters

This endpoint does not need any parameter.

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_mfa_setup

> models::SetupMfaResponse auth_mfa_setup(body)
Generate TOTP secret and setup URI for authenticator enrollment

Generates a cryptographic RFC 6238 TOTP secret, stores it temporarily encrypted in the session cache, and returns an otpauth:// URI and QR code payload compatible with Google Authenticator, 1Password, or Authy. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::SetupMfaResponse**](SetupMFAResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_mfa_verify

> models::AuthTokenResponse auth_mfa_verify(verify_mfa_input)
Verify initial TOTP code to complete MFA enrollment

Validates the initial 6-digit TOTP code against the pending setup secret, sets mfa_enabled to true on auth.users, stores the AES-encrypted secret permanently, and publishes auth.mfa.enabled event. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**verify_mfa_input** | [**VerifyMfaInput**](VerifyMfaInput.md) | Request body for auth.VerifyMFAInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

