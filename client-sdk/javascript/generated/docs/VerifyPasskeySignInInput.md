
# VerifyPasskeySignInInput

VerifyPasskeySignInInput schema

## Properties

Name | Type
------------ | -------------
`authenticatorData` | string
`challenge` | string
`clientDataJson` | string
`credentialId` | string
`signature` | string

## Example

```typescript
import type { VerifyPasskeySignInInput } from ''

// TODO: Update the object below with actual values
const example = {
  "authenticatorData": null,
  "challenge": null,
  "clientDataJson": null,
  "credentialId": null,
  "signature": null,
} satisfies VerifyPasskeySignInInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as VerifyPasskeySignInInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


