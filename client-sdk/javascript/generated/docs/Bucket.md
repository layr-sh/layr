
# Bucket


## Properties

Name | Type
------------ | -------------
`allowedMimeTypes` | Array&lt;string&gt;
`backend` | string
`backendConfig` | { [key: string]: any | null; }
`createdAt` | Date
`id` | any
`isPublic` | boolean
`lastUpdatedAt` | Date
`maxFileSizeBytes` | number
`name` | string

## Example

```typescript
import type { Bucket } from ''

// TODO: Update the object below with actual values
const example = {
  "allowedMimeTypes": null,
  "backend": null,
  "backendConfig": null,
  "createdAt": null,
  "id": null,
  "isPublic": null,
  "lastUpdatedAt": null,
  "maxFileSizeBytes": null,
  "name": null,
} satisfies Bucket

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Bucket
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


