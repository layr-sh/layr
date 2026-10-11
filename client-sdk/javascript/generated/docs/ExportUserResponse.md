
# ExportUserResponse

ExportUserResponse schema

## Properties

Name | Type
------------ | -------------
`exportDate` | string
`identities` | [Array&lt;ExportIdentityRecord&gt;](ExportIdentityRecord.md)
`user` | [User](User.md)

## Example

```typescript
import type { ExportUserResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "exportDate": null,
  "identities": null,
  "user": null,
} satisfies ExportUserResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ExportUserResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


