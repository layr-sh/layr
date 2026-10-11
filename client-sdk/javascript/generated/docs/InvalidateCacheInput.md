
# InvalidateCacheInput

InvalidateCacheInput schema

## Properties

Name | Type
------------ | -------------
`all` | boolean
`catalog` | boolean
`pattern` | string
`schema` | string
`table` | string

## Example

```typescript
import type { InvalidateCacheInput } from ''

// TODO: Update the object below with actual values
const example = {
  "all": null,
  "catalog": null,
  "pattern": null,
  "schema": null,
  "table": null,
} satisfies InvalidateCacheInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as InvalidateCacheInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


