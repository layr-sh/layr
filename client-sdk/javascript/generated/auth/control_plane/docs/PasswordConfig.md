
# PasswordConfig


## Properties

Name | Type
------------ | -------------
`breachCheck` | [PasswordBreachConfig](PasswordBreachConfig.md)
`enabled` | boolean
`minLength` | number
`requireNumbers` | boolean
`requireSymbols` | boolean

## Example

```typescript
import type { PasswordConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "breachCheck": null,
  "enabled": null,
  "minLength": null,
  "requireNumbers": null,
  "requireSymbols": null,
} satisfies PasswordConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as PasswordConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


