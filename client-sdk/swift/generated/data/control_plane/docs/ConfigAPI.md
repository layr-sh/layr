# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**dataConfigGet**](ConfigAPI.md#dataconfigget) | **GET** /v1/_/data/config | Retrieve dynamic data service configuration
[**dataConfigUpdate**](ConfigAPI.md#dataconfigupdate) | **PUT** /v1/_/data/config | Update dynamic data service configuration


# **dataConfigGet**
```swift
    open class func dataConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Retrieve dynamic data service configuration

Returns dynamic data service configuration settings including exposed schema names, connection pool sizes, REST limit defaults, and cache TTL parameters. Requires data:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Retrieve dynamic data service configuration
do {
    let response = try await ConfigAPI.dataConfigGet()
    dump(response)
} catch {
    print(error)
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **dataConfigUpdate**
```swift
    open class func dataConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update dynamic data service configuration

Updates operational data service parameters in data.config, dynamically tuning connection limits, exposed schemas, and cache behaviors without server downtime. Requires data:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(cache: CacheConfig(catalogTtlSeconds: 123, enabled: false, invalidateOnCdc: false, invalidateOnMutation: false, maxCachedQueries: 123, queryTtlSeconds: 123), cors: CORSConfig(allowedHeaders: ["allowedHeaders_example"], allowedOrigins: ["allowedOrigins_example"]), graphql: GraphQLConfig(defaultLimit: 123, enabled: false, introspectionEnabled: false, maxComplexity: 123, maxDepth: 123), rateLimiting: RateLimitingConfig(burst: 123, enabled: false, requestsPerMinute: 123), realtime: RealtimeConfig(enabled: false, heartbeatIntervalMs: 123, maxChannelsPerConnection: 123, maxConnections: 123), rest: RESTConfig(defaultLimit: 123, enabled: false, excludedTables: ["excludedTables_example"], maxLimit: 123), schemas: ["schemas_example"]) // Config | Request body for data.Config

// Update dynamic data service configuration
do {
    let response = try await ConfigAPI.dataConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for data.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

