
# BotProtectionConfig


## Properties

Name | Type
------------ | -------------
`adaptiveFailedAttempts` | number
`enabled` | boolean
`mode` | string
`provider` | string
`secretKey` | string
`secretKeyConfigured` | boolean
`siteKey` | string

## Example

```typescript
import type { BotProtectionConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "adaptiveFailedAttempts": null,
  "enabled": null,
  "mode": null,
  "provider": null,
  "secretKey": null,
  "secretKeyConfigured": null,
  "siteKey": null,
} satisfies BotProtectionConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as BotProtectionConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


