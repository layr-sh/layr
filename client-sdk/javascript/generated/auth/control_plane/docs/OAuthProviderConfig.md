
# OAuthProviderConfig


## Properties

Name | Type
------------ | -------------
`authUrl` | string
`avatarAttribute` | string
`clientId` | string
`clientSecret` | string
`clientSecretConfigured` | boolean
`emailAttribute` | string
`enabled` | boolean
`idAttribute` | string
`nameAttribute` | string
`preset` | string
`responseMode` | string
`responseType` | string
`scope` | string
`tokenUrl` | string
`userinfoUrl` | string

## Example

```typescript
import type { OAuthProviderConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "authUrl": null,
  "avatarAttribute": null,
  "clientId": null,
  "clientSecret": null,
  "clientSecretConfigured": null,
  "emailAttribute": null,
  "enabled": null,
  "idAttribute": null,
  "nameAttribute": null,
  "preset": null,
  "responseMode": null,
  "responseType": null,
  "scope": null,
  "tokenUrl": null,
  "userinfoUrl": null,
} satisfies OAuthProviderConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as OAuthProviderConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


