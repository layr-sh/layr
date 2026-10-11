
# VerifyPasskeySignUpInput

VerifyPasskeySignUpInput schema

## Properties

Name | Type
------------ | -------------
`challenge` | string
`credentialId` | string
`friendlyName` | string
`publicKey` | string
`transports` | Array&lt;string&gt;

## Example

```typescript
import type { VerifyPasskeySignUpInput } from ''

// TODO: Update the object below with actual values
const example = {
  "challenge": null,
  "credentialId": null,
  "friendlyName": null,
  "publicKey": null,
  "transports": null,
} satisfies VerifyPasskeySignUpInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as VerifyPasskeySignUpInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


