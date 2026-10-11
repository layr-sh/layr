# \UserPasskeysApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_passkeys_delete**](UserPasskeysApi.md#auth_user_passkeys_delete) | **DELETE** /v1/auth/user/passkeys/{passkey_id} | Revoke a registered passkey credential
[**auth_user_passkeys_list**](UserPasskeysApi.md#auth_user_passkeys_list) | **GET** /v1/auth/user/passkeys | List registered passkeys for current user



## auth_user_passkeys_delete

> serde_json::Value auth_user_passkeys_delete(passkey_id)
Revoke a registered passkey credential

Deletes a registered WebAuthn passkey credential from auth.passkeys belonging to the authenticated user. Prohibits deleting credentials belonging to other users. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**passkey_id** | **String** |  | [required] |

### Return type

[**serde_json::Value**](serde_json::Value.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


## auth_user_passkeys_list

> Vec<models::Passkey> auth_user_passkeys_list()
List registered passkeys for current user

Retrieves all registered WebAuthn / FIDO2 passkey credentials for the authenticated user from auth.passkeys, including credential IDs, friendly names, creation timestamps, and last used dates. Requires authenticated user session.

### Parameters

This endpoint does not need any parameter.

### Return type

[**Vec<models::Passkey>**](Passkey.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

