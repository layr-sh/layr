
# RateLimitingConfig


## Properties

Name | Type
------------ | -------------
`enabled` | boolean
`lockoutDurationSeconds` | number
`maxSignInAttempts` | number
`windowDurationSeconds` | number

## Example

```typescript
import type { RateLimitingConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "enabled": null,
  "lockoutDurationSeconds": null,
  "maxSignInAttempts": null,
  "windowDurationSeconds": null,
} satisfies RateLimitingConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as RateLimitingConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


