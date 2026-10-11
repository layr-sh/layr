# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authConfigGet**](ConfigAPI.md#authconfigget) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration
[**authConfigUpdate**](ConfigAPI.md#authconfigupdate) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration
[**consoleConfigGet**](ConfigAPI.md#consoleconfigget) | **GET** /v1/_/console/config | Get dynamic runtime console configuration
[**consoleConfigUpdate**](ConfigAPI.md#consoleconfigupdate) | **PUT** /v1/_/console/config | Update dynamic runtime console configuration
[**dataConfigGet**](ConfigAPI.md#dataconfigget) | **GET** /v1/_/data/config | Retrieve dynamic data service configuration
[**dataConfigUpdate**](ConfigAPI.md#dataconfigupdate) | **PUT** /v1/_/data/config | Update dynamic data service configuration
[**fileStorageConfigGet**](ConfigAPI.md#filestorageconfigget) | **GET** /v1/_/file-storage/config | Get runtime file storage configuration
[**fileStorageConfigUpdate**](ConfigAPI.md#filestorageconfigupdate) | **PUT** /v1/_/file-storage/config | Update runtime file storage configuration
[**functionConfigGet**](ConfigAPI.md#functionconfigget) | **GET** /v1/_/function/config | Get runtime function configuration
[**functionConfigUpdate**](ConfigAPI.md#functionconfigupdate) | **PUT** /v1/_/function/config | Update runtime function configuration
[**imageConfigGet**](ConfigAPI.md#imageconfigget) | **GET** /v1/_/image/config | Get runtime image service configuration
[**imageConfigUpdate**](ConfigAPI.md#imageconfigupdate) | **PUT** /v1/_/image/config | Update runtime image service configuration
[**tasksConfigGet**](ConfigAPI.md#tasksconfigget) | **GET** /v1/_/tasks/config | Get runtime tasks orchestrator configuration
[**tasksConfigUpdate**](ConfigAPI.md#tasksconfigupdate) | **PUT** /v1/_/tasks/config | Update runtime tasks orchestrator configuration


# **authConfigGet**
```swift
    open class func authConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Retrieve dynamic runtime authentication configuration

Returns active dynamic authentication settings from auth.config including password strength rules, session TTLs, rate limits, MFA policies, and enabled OAuth providers, with client secrets projected as boolean indicators. Requires auth:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Retrieve dynamic runtime authentication configuration
do {
    let response = try await ConfigAPI.authConfigGet()
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

# **authConfigUpdate**
```swift
    open class func authConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update dynamic runtime authentication configuration

Updates dynamic authentication configuration parameters in auth.config without requiring server restarts. Automatically envelope-encrypts newly supplied third-party OAuth provider secrets with AES-256-GCM using the master encryption key. Requires auth:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for auth.Config

// Update dynamic runtime authentication configuration
do {
    let response = try await ConfigAPI.authConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for auth.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **consoleConfigGet**
```swift
    open class func consoleConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get dynamic runtime console configuration

Returns active runtime console configuration settings including session duration, MFA enforcement policy, and allowed origins. Requires console:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get dynamic runtime console configuration
do {
    let response = try await ConfigAPI.consoleConfigGet()
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

# **consoleConfigUpdate**
```swift
    open class func consoleConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update dynamic runtime console configuration

Mutates runtime console configuration parameters in console.config, validating structure and applying settings dynamically across cluster nodes. Requires console:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for console.Config

// Update dynamic runtime console configuration
do {
    let response = try await ConfigAPI.consoleConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for console.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

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

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for data.Config

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

# **fileStorageConfigGet**
```swift
    open class func fileStorageConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime file storage configuration

Returns dynamic operational configuration settings for the file storage subsystem including global size limits and default backends. Requires storage:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime file storage configuration
do {
    let response = try await ConfigAPI.fileStorageConfigGet()
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

# **fileStorageConfigUpdate**
```swift
    open class func fileStorageConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime file storage configuration

Updates dynamic operational configuration for the file storage subsystem in storage.config, tuning global upload limits and backends without restart. Requires storage:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for filestorage.Config

// Update runtime file storage configuration
do {
    let response = try await ConfigAPI.fileStorageConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for filestorage.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **functionConfigGet**
```swift
    open class func functionConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime function configuration

Returns operational function runtime settings including default isolate timeouts, memory limits, runner concurrency, and isolate pool capacities. Requires function:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime function configuration
do {
    let response = try await ConfigAPI.functionConfigGet()
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

# **functionConfigUpdate**
```swift
    open class func functionConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime function configuration

Modifies operational function runtime settings in function.config, tuning concurrency limits, memory caps, and execution timeouts without restarting the gateway. Requires function:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for function.Config

// Update runtime function configuration
do {
    let response = try await ConfigAPI.functionConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for function.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imageConfigGet**
```swift
    open class func imageConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime image service configuration

Returns operational image service settings including compression quality defaults, maximum resolution limits, allowed source domains, and signature enforcement policy. Requires image:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime image service configuration
do {
    let response = try await ConfigAPI.imageConfigGet()
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

# **imageConfigUpdate**
```swift
    open class func imageConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime image service configuration

Modifies operational image service parameters in image.config, updating compression quality, resolution limits, and domain allowlists without restarting the service. Requires image:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for image.Config

// Update runtime image service configuration
do {
    let response = try await ConfigAPI.imageConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for image.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **tasksConfigGet**
```swift
    open class func tasksConfigGet(apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Get runtime tasks orchestrator configuration

Returns operational task configuration settings including worker concurrency limits, poller interval, default timeout guards, and exponential retry backoff parameters. Requires tasks:config.read scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated


// Get runtime tasks orchestrator configuration
do {
    let response = try await ConfigAPI.tasksConfigGet()
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

# **tasksConfigUpdate**
```swift
    open class func tasksConfigUpdate(config: Config, apiConfiguration: GeneratedAPIConfiguration = GeneratedAPIConfiguration.shared) async throws(ErrorResponse) -> Config
```

Update runtime tasks orchestrator configuration

Updates operational task configuration in tasks.config, dynamically adjusting worker concurrency, polling intervals, and retry backoff limits without restarting workers. Requires tasks:config.write scope.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import Generated

let config = Config(anonymous: AnonymousConfig(enabled: false), cache: CacheConfig(fastPathSessionsEnabled: false, sessionTtlSeconds: 123), emailDispatcher: EmailDispatcherConfig(driver: "driver_example", senderEmail: "senderEmail_example", senderName: "senderName_example", smtp: EmailDispatcherSMTPConfig(host: "host_example", insecureSkipVerify: false, password: "password_example", passwordConfigured: false, port: 123, tlsMode: "tlsMode_example", username: "username_example"), templates: EmailDispatcherTemplatesConfig(emailVerification: EmailDispatcherTemplateConfig(html: "html_example", subject: "subject_example", text: "text_example"), passwordReset: nil, signInOtp: nil, suspiciousActivity: nil), webhook: EmailDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), emailOtp: EmailOTPConfig(enabled: false, tokenExpiryMinutes: 123), mfa: MFAConfig(enabled: false, issuer: "issuer_example", policy: "policy_example", riskTriggers: ["riskTriggers_example"]), oauthProviders: "TODO", oidc: OIDCConfig(clients: [OIDCClientConfig(backchannelSignOutSessionRequired: false, backchannelSignOutUri: "backchannelSignOutUri_example", clientId: "clientId_example", clientSecret: "clientSecret_example", clientSecretConfigured: false, frontchannelSignOutSessionRequired: false, frontchannelSignOutUri: "frontchannelSignOutUri_example", name: "name_example", postSignOutRedirectUris: ["postSignOutRedirectUris_example"], _public: false, redirectUris: ["redirectUris_example"], scopes: ["scopes_example"])], enabled: false, resourceServers: [ResourceServerConfig(description: "description_example", identifier: "identifier_example", name: "name_example", scopes: ["scopes_example"])], ui: OIDCUIConfig(customCss: "customCss_example", logoUrl: "logoUrl_example", privacyPolicyUrl: "privacyPolicyUrl_example", showEmailOtp: false, showOauth: false, showPasskeys: false, showPassword: false, showSignUp: false, showSmsOtp: false, termsOfServiceUrl: "termsOfServiceUrl_example")), passkeys: PasskeysConfig(enabled: false, relyingPartyId: "relyingPartyId_example", relyingPartyName: "relyingPartyName_example"), password: PasswordConfig(breachCheck: PasswordBreachConfig(enabled: false, failOpen: false), enabled: false, minLength: 123, requireNumbers: false, requireSymbols: false), rateLimiting: RateLimitingConfig(enabled: false, lockoutDurationSeconds: 123, maxSignInAttempts: 123, windowDurationSeconds: 123), sessions: SessionsConfig(accessTokenExpirySeconds: 123, idleTimeoutSeconds: 123, refreshTokenExpirySeconds: 123, singleSessionPerUser: false), smsDispatcher: SMSDispatcherConfig(driver: "driver_example", templates: SMSDispatcherTemplatesConfig(passwordReset: SMSDispatcherTemplateConfig(text: "text_example"), phoneVerification: nil, signInOtp: nil), twilio: SMSDispatcherTwilioConfig(accountSid: "accountSid_example", authToken: "authToken_example", authTokenConfigured: false, fromNumber: "fromNumber_example"), webhook: SMSDispatcherWebhookConfig(signingSecret: "signingSecret_example", signingSecretConfigured: false, timeoutSeconds: 123, url: "url_example")), smsOtp: SMSOTPConfig(enabled: false, tokenExpiryMinutes: 123), threat: ThreatConfig(botProtection: BotProtectionConfig(adaptiveFailedAttempts: 123, enabled: false, mode: "mode_example", provider: "provider_example", secretKey: "secretKey_example", secretKeyConfigured: false, siteKey: "siteKey_example"), knownDevicesMaxDays: 123, notifyOnNewDevice: false)) // Config | Request body for tasks.Config

// Update runtime tasks orchestrator configuration
do {
    let response = try await ConfigAPI.tasksConfigUpdate(config: config)
    dump(response)
} catch {
    print(error)
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **config** | [**Config**](Config.md) | Request body for tasks.Config | 

### Return type

[**Config**](Config.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json, application/xml

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

