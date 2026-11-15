
# SetMultipleKVInput

SetMultipleKVInput schema

## Properties

Name | Type
------------ | -------------
`entries` | { [key: string]: string; }
`ttl` | number

## Example

```typescript
import type { SetMultipleKVInput } from ''

// TODO: Update the object below with actual values
const example = {
  "entries": null,
  "ttl": null,
} satisfies SetMultipleKVInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SetMultipleKVInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


