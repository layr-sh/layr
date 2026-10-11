
# Config

Config schema

## Properties

Name | Type
------------ | -------------
`anonymous` | [AnonymousConfig](AnonymousConfig.md)
`cache` | [CacheConfig](CacheConfig.md)
`emailDispatcher` | [EmailDispatcherConfig](EmailDispatcherConfig.md)
`emailOtp` | [EmailOTPConfig](EmailOTPConfig.md)
`mfa` | [MFAConfig](MFAConfig.md)
`oauthProviders` | [{ [key: string]: OAuthProviderConfig; }](OAuthProviderConfig.md)
`oidc` | [OIDCConfig](OIDCConfig.md)
`passkeys` | [PasskeysConfig](PasskeysConfig.md)
`password` | [PasswordConfig](PasswordConfig.md)
`rateLimiting` | [RateLimitingConfig](RateLimitingConfig.md)
`sessions` | [SessionsConfig](SessionsConfig.md)
`smsDispatcher` | [SMSDispatcherConfig](SMSDispatcherConfig.md)
`smsOtp` | [SMSOTPConfig](SMSOTPConfig.md)
`threat` | [ThreatConfig](ThreatConfig.md)

## Example

```typescript
import type { Config } from ''

// TODO: Update the object below with actual values
const example = {
  "anonymous": null,
  "cache": null,
  "emailDispatcher": null,
  "emailOtp": null,
  "mfa": null,
  "oauthProviders": null,
  "oidc": null,
  "passkeys": null,
  "password": null,
  "rateLimiting": null,
  "sessions": null,
  "smsDispatcher": null,
  "smsOtp": null,
  "threat": null,
} satisfies Config

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Config
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


