
# ProviderStats


## Properties

Name | Type
------------ | -------------
`identityCount` | number
`lastSignedInAt` | Date
`provider` | string
`userCount` | number

## Example

```typescript
import type { ProviderStats } from ''

// TODO: Update the object below with actual values
const example = {
  "identityCount": null,
  "lastSignedInAt": null,
  "provider": null,
  "userCount": null,
} satisfies ProviderStats

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ProviderStats
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


