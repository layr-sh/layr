
# OIDCUIConfig


## Properties

Name | Type
------------ | -------------
`customCss` | string
`logoUrl` | string
`privacyPolicyUrl` | string
`showEmailOtp` | boolean
`showOauth` | boolean
`showPasskeys` | boolean
`showPassword` | boolean
`showSignUp` | boolean
`showSmsOtp` | boolean
`termsOfServiceUrl` | string

## Example

```typescript
import type { OIDCUIConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "customCss": null,
  "logoUrl": null,
  "privacyPolicyUrl": null,
  "showEmailOtp": null,
  "showOauth": null,
  "showPasskeys": null,
  "showPassword": null,
  "showSignUp": null,
  "showSmsOtp": null,
  "termsOfServiceUrl": null,
} satisfies OIDCUIConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as OIDCUIConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


