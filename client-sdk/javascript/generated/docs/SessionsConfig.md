
# SessionsConfig


## Properties

Name | Type
------------ | -------------
`accessTokenExpirySeconds` | number
`idleTimeoutSeconds` | number
`refreshTokenExpirySeconds` | number
`singleSessionPerUser` | boolean

## Example

```typescript
import type { SessionsConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "accessTokenExpirySeconds": null,
  "idleTimeoutSeconds": null,
  "refreshTokenExpirySeconds": null,
  "singleSessionPerUser": null,
} satisfies SessionsConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SessionsConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


