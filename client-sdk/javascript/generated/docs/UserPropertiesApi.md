# UserPropertiesApi

All URIs are relative to *http://localhost*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**authUserPropertiesUpdate**](UserPropertiesApi.md#authuserpropertiesupdate) | **PATCH** /v1/auth/user/properties | Update custom personal properties on user profile |



## authUserPropertiesUpdate

> UpdateUserPropertiesResponse authUserPropertiesUpdate(updateUserPropertiesInput)

Update custom personal properties on user profile

Performs shallow merge of custom JSON properties into the properties column of auth.users for the authenticated user, validating JSON size limits. Requires authenticated user session.

### Example

```ts
import {
  Configuration,
  UserPropertiesApi,
} from '';
import type { AuthUserPropertiesUpdateRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new UserPropertiesApi();

  const body = {
    // UpdateUserPropertiesInput | Request body for auth.UpdateUserPropertiesInput
    updateUserPropertiesInput: ...,
  } satisfies AuthUserPropertiesUpdateRequest;

  try {
    const data = await api.authUserPropertiesUpdate(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **updateUserPropertiesInput** | [UpdateUserPropertiesInput](UpdateUserPropertiesInput.md) | Request body for auth.UpdateUserPropertiesInput | |

### Return type

[**UpdateUserPropertiesResponse**](UpdateUserPropertiesResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`, `application/xml`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

