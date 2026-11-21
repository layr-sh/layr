# StatsAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authStatsGet**](StatsAPI.md#authstatsget) | **GET** /v1/_/auth/stats | Get application authentication statistics


# **authStatsGet**
```swift
    open class func authStatsGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> StatsResponse
```

Get application authentication statistics

Returns directory-wide application user, credential, and session counts from a consistent database snapshot. Active sessions exclude expired, idle, and locked-user sessions. Requires auth:stats.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get application authentication statistics
do {
    let response = try await StatsAPI.authStatsGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**StatsResponse**](StatsResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

