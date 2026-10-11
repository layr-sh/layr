
# ExportIdentityRecord


## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`lastSignInAt` | Date
`properties` | { [key: string]: any | null; }
`provider` | string
`providerUserId` | string

## Example

```typescript
import type { ExportIdentityRecord } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "lastSignInAt": null,
  "properties": null,
  "provider": null,
  "providerUserId": null,
} satisfies ExportIdentityRecord

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ExportIdentityRecord
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


