# \PasskeysSignInApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_passkeys_sign_in_begin**](PasskeysSignInApi.md#auth_passkeys_sign_in_begin) | **POST** /v1/auth/passkeys/sign-in | Begin WebAuthn passkey authentication ceremony
[**auth_passkeys_sign_in_verify**](PasskeysSignInApi.md#auth_passkeys_sign_in_verify) | **POST** /v1/auth/passkeys/sign-in/verify | Verify WebAuthn passkey assertion signature and issue session



## auth_passkeys_sign_in_begin

> models::SignInOptions auth_passkeys_sign_in_begin(body)
Begin WebAuthn passkey authentication ceremony

Initiates WebAuthn / FIDO2 assertion ceremony, generating a cryptographic challenge and credential request options for the client browser or authenticator hardware.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**body** | Option<**serde_json::Value**> | Request body for core.Empty | [required] |

### Return type

[**models::SignInOptions**](SignInOptions.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_passkeys_sign_in_verify

> models::AuthTokenResponse auth_passkeys_sign_in_verify(verify_passkey_sign_in_input)
Verify WebAuthn passkey assertion signature and issue session

Verifies the client's WebAuthn assertion signature against the registered public key and signature counter stored in auth.passkeys. Issues an Ed25519 JWT access token, refresh token, and session record in auth.sessions.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**verify_passkey_sign_in_input** | [**VerifyPasskeySignInInput**](VerifyPasskeySignInInput.md) | Request body for auth.VerifyPasskeySignInInput | [required] |

### Return type

[**models::AuthTokenResponse**](AuthTokenResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

