
# ListExecutionsResponse

ListExecutionsResponse schema

## Properties

Name | Type
------------ | -------------
`count` | number
`executions` | [Array&lt;Execution&gt;](Execution.md)

## Example

```typescript
import type { ListExecutionsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "count": null,
  "executions": null,
} satisfies ListExecutionsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListExecutionsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


