
# Session


## Properties

Name | Type
------------ | -------------
`clientId` | string
`createdAt` | Date
`expiresAt` | Date
`id` | string
`ipAddress` | string
`refreshTokenHash` | string
`userAgent` | string
`userId` | string

## Example

```typescript
import type { Session } from ''

// TODO: Update the object below with actual values
const example = {
  "clientId": null,
  "createdAt": null,
  "expiresAt": null,
  "id": null,
  "ipAddress": null,
  "refreshTokenHash": null,
  "userAgent": null,
  "userId": null,
} satisfies Session

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Session
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


