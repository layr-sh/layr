
# ExecuteGraphQLInput

ExecuteGraphQLInput schema

## Properties

Name | Type
------------ | -------------
`operationName` | string
`query` | string
`variables` | { [key: string]: any | null; }

## Example

```typescript
import type { ExecuteGraphQLInput } from ''

// TODO: Update the object below with actual values
const example = {
  "operationName": null,
  "query": null,
  "variables": null,
} satisfies ExecuteGraphQLInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ExecuteGraphQLInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


