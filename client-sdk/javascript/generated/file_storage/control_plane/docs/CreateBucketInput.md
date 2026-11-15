
# CreateBucketInput

CreateBucketInput schema

## Properties

Name | Type
------------ | -------------
`allowedMimeTypes` | Array&lt;string&gt;
`backend` | string
`backendConfig` | { [key: string]: any | null; }
`isPublic` | boolean
`maxFileSizeBytes` | number
`name` | string

## Example

```typescript
import type { CreateBucketInput } from ''

// TODO: Update the object below with actual values
const example = {
  "allowedMimeTypes": null,
  "backend": null,
  "backendConfig": null,
  "isPublic": null,
  "maxFileSizeBytes": null,
  "name": null,
} satisfies CreateBucketInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateBucketInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


