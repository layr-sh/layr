
# CreateIndexInput

CreateIndexInput schema

## Properties

Name | Type
------------ | -------------
`columns` | Array&lt;string&gt;
`indexName` | string
`isUnique` | boolean
`type` | string

## Example

```typescript
import type { CreateIndexInput } from ''

// TODO: Update the object below with actual values
const example = {
  "columns": null,
  "indexName": null,
  "isUnique": null,
  "type": null,
} satisfies CreateIndexInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateIndexInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


