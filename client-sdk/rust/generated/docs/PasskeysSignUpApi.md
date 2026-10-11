# \PasskeysSignUpApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_passkeys_sign_up_begin**](PasskeysSignUpApi.md#auth_passkeys_sign_up_begin) | **POST** /v1/auth/passkeys/sign-up | Begin WebAuthn passkey registration ceremony
[**auth_passkeys_sign_up_verify**](PasskeysSignUpApi.md#auth_passkeys_sign_up_verify) | **POST** /v1/auth/passkeys/sign-up/verify | Verify WebAuthn passkey registration attestation and store credential



## auth_passkeys_sign_up_begin

> models::SignUpOptions auth_passkeys_sign_up_begin(begin_passkey_sign_up_input)
Begin WebAuthn passkey registration ceremony

Initiates WebAuthn / FIDO2 credential creation ceremony, generating an attestation challenge, relying party metadata, and user entity options for hardware or platform passkey enrollment.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**begin_passkey_sign_up_input** | [**BeginPasskeySignUpInput**](BeginPasskeySignUpInput.md) | Request body for auth.BeginPasskeySignUpInput | [required] |

### Return type

[**models::SignUpOptions**](SignUpOptions.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_passkeys_sign_up_verify

> models::AuthTokenResponse auth_passkeys_sign_up_verify(verify_passkey_sign_up_input)
Verify WebAuthn passkey registration attestation and store credential

Validates client attestation response and public key, stores credential record in auth.passkeys, creates user identity, and issues an authenticated session with Ed25519 JWT tokens.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**verify_passkey_sign_up_input** | [**VerifyPasskeySignUpInput**](VerifyPasskeySignUpInput.md) | Request body for auth.VerifyPasskeySignUpInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

