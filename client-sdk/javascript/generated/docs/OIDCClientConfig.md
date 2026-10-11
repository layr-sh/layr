
# OIDCClientConfig


## Properties

Name | Type
------------ | -------------
`backchannelSignOutSessionRequired` | boolean
`backchannelSignOutUri` | string
`clientId` | string
`clientSecret` | string
`clientSecretConfigured` | boolean
`frontchannelSignOutSessionRequired` | boolean
`frontchannelSignOutUri` | string
`name` | string
`postSignOutRedirectUris` | Array&lt;string&gt;
`_public` | boolean
`redirectUris` | Array&lt;string&gt;
`scopes` | Array&lt;string&gt;

## Example

```typescript
import type { OIDCClientConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "backchannelSignOutSessionRequired": null,
  "backchannelSignOutUri": null,
  "clientId": null,
  "clientSecret": null,
  "clientSecretConfigured": null,
  "frontchannelSignOutSessionRequired": null,
  "frontchannelSignOutUri": null,
  "name": null,
  "postSignOutRedirectUris": null,
  "_public": null,
  "redirectUris": null,
  "scopes": null,
} satisfies OIDCClientConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as OIDCClientConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


