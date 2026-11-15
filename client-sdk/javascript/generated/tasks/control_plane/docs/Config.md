
# Config

Config schema

## Properties

Name | Type
------------ | -------------
`concurrencyLimit` | number
`pollIntervalMs` | number
`retryInitialDelaySeconds` | number
`retryMaxAttempts` | number
`retryMaxDelaySeconds` | number
`timeoutSeconds` | number

## Example

```typescript
import type { Config } from ''

// TODO: Update the object below with actual values
const example = {
  "concurrencyLimit": null,
  "pollIntervalMs": null,
  "retryInitialDelaySeconds": null,
  "retryMaxAttempts": null,
  "retryMaxDelaySeconds": null,
  "timeoutSeconds": null,
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


