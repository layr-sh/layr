
# BucketUsage


## Properties

Name | Type
------------ | -------------
`backend` | string
`isPublic` | boolean
`lastUpdatedAt` | Date
`name` | string
`objectCount` | number
`sizeBytes` | number

## Example

```typescript
import type { BucketUsage } from ''

// TODO: Update the object below with actual values
const example = {
  "backend": null,
  "isPublic": null,
  "lastUpdatedAt": null,
  "name": null,
  "objectCount": null,
  "sizeBytes": null,
} satisfies BucketUsage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as BucketUsage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


