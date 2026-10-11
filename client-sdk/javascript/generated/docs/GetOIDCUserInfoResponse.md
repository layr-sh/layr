
# GetOIDCUserInfoResponse

GetOIDCUserInfoResponse schema

## Properties

Name | Type
------------ | -------------
`email` | string
`emailVerified` | boolean
`isAnonymous` | boolean
`name` | string
`phoneNumber` | string
`phoneNumberVerified` | boolean
`properties` | { [key: string]: any | null; }
`role` | string
`sub` | string
`updatedAt` | number

## Example

```typescript
import type { GetOIDCUserInfoResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "email": null,
  "emailVerified": null,
  "isAnonymous": null,
  "name": null,
  "phoneNumber": null,
  "phoneNumberVerified": null,
  "properties": null,
  "role": null,
  "sub": null,
  "updatedAt": null,
} satisfies GetOIDCUserInfoResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetOIDCUserInfoResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


