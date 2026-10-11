
# SetKVResponse

SetKVResponse schema

## Properties

Name | Type
------------ | -------------
`created` | boolean
`key` | string
`status` | string
`ttl` | number

## Example

```typescript
import type { SetKVResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "created": null,
  "key": null,
  "status": null,
  "ttl": null,
} satisfies SetKVResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SetKVResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


