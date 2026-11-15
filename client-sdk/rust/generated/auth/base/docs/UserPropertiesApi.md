# \UserPropertiesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**auth_user_properties_update**](UserPropertiesApi.md#auth_user_properties_update) | **PATCH** /v1/auth/user/properties | Update custom personal properties on user profile



## auth_user_properties_update

> models::UpdateUserPropertiesResponse auth_user_properties_update(update_user_properties_input)
Update custom personal properties on user profile

Performs shallow merge of custom JSON properties into the properties column of auth.users for the authenticated user, validating JSON size limits. Requires authenticated user session.

### Parameters


Name | Type | Description  | Required | Notes
------------- | ------------- | ------------- | ------------- | -------------
**update_user_properties_input** | [**UpdateUserPropertiesInput**](UpdateUserPropertiesInput.md) | Request body for auth.UpdateUserPropertiesInput | [required] |

### Return type

[**models::UpdateUserPropertiesResponse**](UpdateUserPropertiesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

