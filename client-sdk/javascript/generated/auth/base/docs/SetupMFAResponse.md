
# SetupMFAResponse

SetupMFAResponse schema

## Properties

Name | Type
------------ | -------------
`authUrl` | string
`digits` | number
`issuer` | string
`periodSeconds` | number
`secret` | string

## Example

```typescript
import type { SetupMFAResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "authUrl": null,
  "digits": null,
  "issuer": null,
  "periodSeconds": null,
  "secret": null,
} satisfies SetupMFAResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SetupMFAResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


