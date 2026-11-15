
# GetUserResponse

GetUserResponse schema

## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`email` | string
`emailVerified` | boolean
`id` | string
`isAnonymous` | boolean
`lastUpdatedAt` | Date
`mfaEnabled` | boolean
`phone` | string
`phoneVerified` | boolean
`properties` | { [key: string]: any | null; }
`role` | string

## Example

```typescript
import type { GetUserResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "email": null,
  "emailVerified": null,
  "id": null,
  "isAnonymous": null,
  "lastUpdatedAt": null,
  "mfaEnabled": null,
  "phone": null,
  "phoneVerified": null,
  "properties": null,
  "role": null,
} satisfies GetUserResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetUserResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


