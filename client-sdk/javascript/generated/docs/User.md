
# User


## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`email` | string
`emailVerifiedAt` | Date
`id` | string
`isAnonymous` | boolean
`lastUpdatedAt` | Date
`lockedUntil` | Date
`mfaEnabled` | boolean
`phone` | string
`phoneVerifiedAt` | Date
`properties` | { [key: string]: any | null; }
`role` | string

## Example

```typescript
import type { User } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "email": null,
  "emailVerifiedAt": null,
  "id": null,
  "isAnonymous": null,
  "lastUpdatedAt": null,
  "lockedUntil": null,
  "mfaEnabled": null,
  "phone": null,
  "phoneVerifiedAt": null,
  "properties": null,
  "role": null,
} satisfies User

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as User
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


