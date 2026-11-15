
# AuthTokenResponse

AuthTokenResponse schema

## Properties

Name | Type
------------ | -------------
`accessToken` | string
`claims` | { [key: string]: any | null; }
`expiresIn` | number
`refreshToken` | string
`tokenType` | string
`user` | [User](User.md)

## Example

```typescript
import type { AuthTokenResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "accessToken": null,
  "claims": null,
  "expiresIn": null,
  "refreshToken": null,
  "tokenType": null,
  "user": null,
} satisfies AuthTokenResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as AuthTokenResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


