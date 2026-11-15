# ConfigAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**authConfigGet**](ConfigAPI.md#authconfigget) | **GET** /v1/_/auth/config | Retrieve dynamic runtime authentication configuration
[**authConfigUpdate**](ConfigAPI.md#authconfigupdate) | **PUT** /v1/_/auth/config | Update dynamic runtime authentication configuration


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

