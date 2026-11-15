
# ModelObject


## Properties

Name | Type
------------ | -------------
`bucketId` | any
`checksumSha256` | string
`contentType` | string
`createdAt` | Date
`id` | any
`lastUpdatedAt` | Date
`metadata` | { [key: string]: any | null; }
`objectKey` | string
`sizeBytes` | number

## Example

```typescript
import type { ModelObject } from ''

// TODO: Update the object below with actual values
const example = {
  "bucketId": null,
  "checksumSha256": null,
  "contentType": null,
  "createdAt": null,
  "id": null,
  "lastUpdatedAt": null,
  "metadata": null,
  "objectKey": null,
  "sizeBytes": null,
} satisfies ModelObject

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ModelObject
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


